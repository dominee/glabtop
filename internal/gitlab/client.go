package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"glabtop/internal/model"
)

// MaxListItems caps list payloads shown in the TUI per type.
const MaxListItems = 200

// maxTimelinePages caps pagination when pulling full history for charts.
const maxTimelinePages = 400

type bucketAgg struct {
	mu sync.Mutex
	c  map[int64]int
	m  map[int64]int
	i  map[int64]int
}

func newBucketAgg() *bucketAgg {
	return &bucketAgg{
		c: make(map[int64]int),
		m: make(map[int64]int),
		i: make(map[int64]int),
	}
}

func (a *bucketAgg) add(tr model.TimeRange, kind byte, ts time.Time) {
	k := model.BucketStart(ts, tr).Unix()
	a.mu.Lock()
	defer a.mu.Unlock()
	switch kind {
	case 'c':
		a.c[k]++
	case 'm':
		a.m[k]++
	case 'i':
		a.i[k]++
	}
}

func (a *bucketAgg) maps() (commits, merges, issues map[int64]int) {
	return a.c, a.m, a.i
}

type Client struct {
	base    *url.URL
	token   string
	http    *http.Client
	perPage int
}

func NewClient(host, token string) (*Client, error) {
	host = strings.TrimRight(host, "/")
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		return nil, fmt.Errorf("gitlab host must include scheme (https://)")
	}
	return &Client{
		base:    u,
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second},
		perPage: 100,
	}, nil
}

func (c *Client) apiURL(p string, q url.Values) string {
	ref := &url.URL{Scheme: c.base.Scheme, Host: c.base.Host, Path: "/api/v4" + p}
	if len(q) > 0 {
		ref.RawQuery = q.Encode()
	}
	return ref.String()
}

func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	endpoint := c.apiURL(path, q)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("gitlab %s: %s — %s", res.Status, endpoint, truncate(string(body), 200))
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ResolveProjects expands group paths and explicit project paths.
func (c *Client) ResolveProjects(ctx context.Context, groups, projects []string) ([]model.ProjectRef, error) {
	seen := make(map[int]struct{})
	var out []model.ProjectRef
	var mu sync.Mutex
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	setErr := func(e error) {
		if e == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}

	for _, p := range projects {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			enc := url.PathEscape(p)
			b, err := c.get(ctx, "/projects/"+enc, nil)
			if err != nil {
				setErr(fmt.Errorf("project %q: %w", p, err))
				return
			}
			var pr struct {
				ID                int    `json:"id"`
				PathWithNamespace string `json:"path_with_namespace"`
			}
			if err := json.Unmarshal(b, &pr); err != nil {
				setErr(err)
				return
			}
			mu.Lock()
			if _, ok := seen[pr.ID]; !ok {
				seen[pr.ID] = struct{}{}
				out = append(out, model.ProjectRef{ID: pr.ID, PathWithNamespace: pr.PathWithNamespace})
			}
			mu.Unlock()
		}()
	}

	for _, g := range groups {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			enc := url.PathEscape(g)
			page := 1
			for {
				q := url.Values{}
				q.Set("per_page", strconv.Itoa(c.perPage))
				q.Set("page", strconv.Itoa(page))
				q.Set("include_subgroups", "true")
				b, err := c.get(ctx, "/groups/"+enc+"/projects", q)
				if err != nil {
					setErr(fmt.Errorf("group %q: %w", g, err))
					return
				}
				var chunk []struct {
					ID                int    `json:"id"`
					PathWithNamespace string `json:"path_with_namespace"`
				}
				if err := json.Unmarshal(b, &chunk); err != nil {
					setErr(err)
					return
				}
				mu.Lock()
				for _, pr := range chunk {
					if _, ok := seen[pr.ID]; ok {
						continue
					}
					seen[pr.ID] = struct{}{}
					out = append(out, model.ProjectRef{ID: pr.ID, PathWithNamespace: pr.PathWithNamespace})
				}
				mu.Unlock()
				if len(chunk) < c.perPage {
					break
				}
				page++
			}
		}()
	}

	wg.Wait()
	if firstErr != nil {
		return out, firstErr
	}
	return out, nil
}

// FetchSnapshot loads list data and a time-bucketed series for the chart.
func (c *Client) FetchSnapshot(ctx context.Context, projects []model.ProjectRef, tr model.TimeRange, projectSub, userSub string) (*model.Snapshot, error) {
	now := time.Now().UTC()
	since, until := model.WindowBounds(tr, now)
	winID := model.WindowID(now, tr)
	sSince := since.Format(time.RFC3339)
	sUntil := until.Format(time.RFC3339)

	globalAgg := newBucketAgg()
	var (
		allCommits []model.CommitRow
		allIssues  []model.IssueRow
		mu         sync.Mutex
		wg         sync.WaitGroup
		sem        = make(chan struct{}, 5)
		firstErr   error
		errMu      sync.Mutex
	)

	setErr := func(e error) {
		if e == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = e
		}
		errMu.Unlock()
	}

	userMatch := func(name string) bool {
		if userSub == "" {
			return true
		}
		return strings.Contains(strings.ToLower(name), strings.ToLower(userSub))
	}

	for _, p := range projects {
		if projectSub != "" && !strings.Contains(strings.ToLower(p.PathWithNamespace), strings.ToLower(projectSub)) {
			continue
		}
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			commits, err := c.fetchCommits(ctx, p, sSince, sUntil, 0)
			if err != nil {
				setErr(fmt.Errorf("%s commits: %w", p.PathWithNamespace, err))
				return
			}
			localAgg := newBucketAgg()
			var filteredCommits []model.CommitRow
			for _, cm := range commits {
				if !userMatch(cm.AuthorName) && !userMatch(cm.AuthorEmail) {
					continue
				}
				localAgg.add(tr, 'c', cm.CreatedAt)
				cm.ProjectPath = p.PathWithNamespace
				filteredCommits = append(filteredCommits, cm)
			}
			if err := c.collectMergedMRs(ctx, p, since, until, userSub, tr, localAgg); err != nil {
				setErr(fmt.Errorf("%s merge_requests: %w", p.PathWithNamespace, err))
				return
			}
			issues, err := c.fetchClosedIssues(ctx, p, sSince, sUntil, 0)
			if err != nil {
				setErr(fmt.Errorf("%s issues: %w", p.PathWithNamespace, err))
				return
			}
			var filteredIssues []model.IssueRow
			for _, is := range issues {
				if !userMatch(is.AuthorName) && !userMatch(is.AssigneeName) {
					continue
				}
				localAgg.add(tr, 'i', is.ClosedAt)
				is.ProjectPath = p.PathWithNamespace
				filteredIssues = append(filteredIssues, is)
			}

			mu.Lock()
			allCommits = append(allCommits, filteredCommits...)
			allIssues = append(allIssues, filteredIssues...)
			mergeMaps(globalAgg, localAgg)
			mu.Unlock()
		}()
	}

	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	cArr, mArr, iArr := globalAgg.maps()
	series := model.SeriesFromMaps(since, until, tr, cArr, mArr, iArr)
	counts := model.NormalizeSeries(series)

	model.SortCommitsByTimeDesc(allCommits)
	model.SortIssuesByTimeDesc(allIssues)
	trimSortCommits(&allCommits)
	trimSortIssues(&allIssues)

	snap := &model.Snapshot{
		WindowID:     winID,
		TimeRange:    tr.String(),
		SinceRFC3339: sSince,
		UntilRFC3339: sUntil,
		FetchedUnix:  time.Now().Unix(),
		Stale:        false,
		Series:       series,
		Commits:      allCommits,
		Issues:       allIssues,
		Counts:       counts,
	}
	return snap, nil
}

func mergeMaps(dst, src *bucketAgg) {
	sc, sm, si := src.maps()
	dst.mu.Lock()
	defer dst.mu.Unlock()
	for k, v := range sc {
		dst.c[k] += v
	}
	for k, v := range sm {
		dst.m[k] += v
	}
	for k, v := range si {
		dst.i[k] += v
	}
}

func trimSortCommits(commits *[]model.CommitRow) {
	if len(*commits) > MaxListItems {
		*commits = (*commits)[:MaxListItems]
	}
}

func trimSortIssues(issues *[]model.IssueRow) {
	if len(*issues) > MaxListItems {
		*issues = (*issues)[:MaxListItems]
	}
}

func (c *Client) fetchCommits(ctx context.Context, p model.ProjectRef, since, until string, maxRows int) ([]model.CommitRow, error) {
	enc := strconv.Itoa(p.ID)
	var out []model.CommitRow
	page := 1
	for page <= maxTimelinePages {
		if maxRows > 0 && len(out) >= maxRows {
			break
		}
		q := url.Values{}
		q.Set("since", since)
		q.Set("until", until)
		q.Set("per_page", strconv.Itoa(c.perPage))
		q.Set("page", strconv.Itoa(page))
		b, err := c.get(ctx, "/projects/"+enc+"/repository/commits", q)
		if err != nil {
			return out, err
		}
		var chunk []struct {
			ID             string     `json:"id"`
			ShortID        string     `json:"short_id"`
			Title          string     `json:"title"`
			AuthorName     string     `json:"author_name"`
			AuthorEmail    string     `json:"author_email"`
			CreatedAt      time.Time  `json:"created_at"`
			CommittedDate  *time.Time `json:"committed_date"`
			LastModifiedAt *time.Time `json:"last_modified_at"`
		}
		if err := json.Unmarshal(b, &chunk); err != nil {
			return out, err
		}
		for _, raw := range chunk {
			t := raw.CreatedAt
			if raw.CommittedDate != nil {
				t = *raw.CommittedDate
			} else if raw.LastModifiedAt != nil {
				t = *raw.LastModifiedAt
			}
			out = append(out, model.CommitRow{
				ID: raw.ID, ShortID: raw.ShortID, Title: raw.Title,
				AuthorName: raw.AuthorName, AuthorEmail: raw.AuthorEmail, CreatedAt: t,
			})
			if maxRows > 0 && len(out) >= maxRows {
				break
			}
		}
		if len(chunk) < c.perPage {
			break
		}
		page++
	}
	return out, nil
}

func (c *Client) collectMergedMRs(ctx context.Context, p model.ProjectRef, since, until time.Time, userSub string, tr model.TimeRange, agg *bucketAgg) error {
	enc := strconv.Itoa(p.ID)
	page := 1
	uMatch := strings.ToLower(userSub)
	for page <= maxTimelinePages {
		q := url.Values{}
		q.Set("state", "merged")
		q.Set("per_page", strconv.Itoa(c.perPage))
		q.Set("page", strconv.Itoa(page))
		q.Set("order_by", "updated_at")
		q.Set("sort", "desc")
		b, err := c.get(ctx, "/projects/"+enc+"/merge_requests", q)
		if err != nil {
			return err
		}
		var chunk []struct {
			MergedAt *time.Time `json:"merged_at"`
			Author   struct {
				Name string `json:"name"`
			} `json:"author"`
			UpdatedAt time.Time `json:"updated_at"`
		}
		if err := json.Unmarshal(b, &chunk); err != nil {
			return err
		}
		if len(chunk) == 0 {
			break
		}
		stopPaging := false
		for _, mr := range chunk {
			mt := mr.UpdatedAt
			if mr.MergedAt != nil {
				mt = *mr.MergedAt
			}
			if mt.Before(since) {
				stopPaging = true
				break
			}
			if mr.MergedAt == nil {
				continue
			}
			m := *mr.MergedAt
			if !m.Before(since) && !m.After(until) {
				if userSub != "" && !strings.Contains(strings.ToLower(mr.Author.Name), uMatch) {
					continue
				}
				agg.add(tr, 'm', m)
			}
		}
		if stopPaging || len(chunk) < c.perPage {
			break
		}
		page++
	}
	return nil
}

func (c *Client) fetchClosedIssues(ctx context.Context, p model.ProjectRef, since, until string, maxRows int) ([]model.IssueRow, error) {
	sinceT, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return nil, err
	}
	untilT, err := time.Parse(time.RFC3339, until)
	if err != nil {
		return nil, err
	}
	enc := strconv.Itoa(p.ID)
	var out []model.IssueRow
	page := 1
	for page <= maxTimelinePages {
		if maxRows > 0 && len(out) >= maxRows {
			break
		}
		q := url.Values{}
		q.Set("state", "closed")
		q.Set("updated_after", since)
		q.Set("updated_before", until)
		q.Set("per_page", strconv.Itoa(c.perPage))
		q.Set("page", strconv.Itoa(page))
		q.Set("order_by", "updated_at")
		q.Set("sort", "desc")
		b, err := c.get(ctx, "/projects/"+enc+"/issues", q)
		if err != nil {
			return out, err
		}
		var chunk []struct {
			IID       int        `json:"iid"`
			Title     string     `json:"title"`
			State     string     `json:"state"`
			WebURL    string     `json:"web_url"`
			ClosedAt  *time.Time `json:"closed_at"`
			UpdatedAt time.Time  `json:"updated_at"`
			Author    struct {
				Name string `json:"name"`
			} `json:"author"`
			Assignee *struct {
				Name string `json:"name"`
			} `json:"assignee"`
		}
		if err := json.Unmarshal(b, &chunk); err != nil {
			return out, err
		}
		if len(chunk) == 0 {
			break
		}
		for _, raw := range chunk {
			if raw.ClosedAt == nil {
				continue
			}
			cl := *raw.ClosedAt
			if cl.Before(sinceT) || cl.After(untilT) {
				continue
			}
			row := model.IssueRow{
				IID: raw.IID, Title: raw.Title, State: raw.State,
				AuthorName: raw.Author.Name, ClosedAt: cl, UpdatedAt: raw.UpdatedAt,
				WebURL: raw.WebURL,
			}
			if raw.Assignee != nil {
				row.AssigneeName = raw.Assignee.Name
			}
			out = append(out, row)
			if maxRows > 0 && len(out) >= maxRows {
				return out, nil
			}
		}
		if len(chunk) < c.perPage {
			break
		}
		page++
	}
	return out, nil
}
