package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/docker/docker/api/types/image"
	"github.com/vmsilvamolina/yaktui/internal/dockerclient"
)

// imageRow is one displayed row: an image summary paired with the single
// repo:tag it represents (an image can have multiple RepoTags, or none).
type imageRow struct {
	summary    image.Summary
	repository string
	tag        string
}

// ImagesModel represents the Docker images view
type ImagesModel struct {
	client        *dockerclient.Client
	table         table.Model
	rows          []imageRow
	width         int
	height        int
	loading       bool
	filterQuery   string
	filteredCount int
}

// NewImagesModel creates a new images model
func NewImagesModel(c *dockerclient.Client) *ImagesModel {
	columns := []table.Column{
		{Title: "REPOSITORY", Width: 30},
		{Title: "TAG", Width: 15},
		{Title: "ID", Width: 15},
		{Title: "SIZE", Width: 10},
		{Title: "CREATED", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(podTableStylesNormal())

	return &ImagesModel{client: c, table: t, loading: true}
}

// Init initializes the model
func (m *ImagesModel) Init() tea.Cmd {
	return m.fetchImages
}

// SetSize sets the view size
func (m *ImagesModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.table.SetHeight(height - 4)

	totalWidth := width - 10
	columns := []table.Column{
		{Title: "REPOSITORY", Width: totalWidth * 35 / 100},
		{Title: "TAG", Width: totalWidth * 20 / 100},
		{Title: "ID", Width: totalWidth * 15 / 100},
		{Title: "SIZE", Width: totalWidth * 15 / 100},
		{Title: "CREATED", Width: totalWidth * 15 / 100},
	}
	m.table.SetColumns(columns)
}

// Update handles messages
func (m *ImagesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case ImagesMsg:
		m.rows = buildImageRows(msg.Images)
		m.loading = false
		m.updateTable()
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the model
func (m *ImagesModel) View() string {
	if m.loading {
		return "Loading..."
	}
	return m.table.View()
}

// Refresh fetches fresh data
func (m *ImagesModel) Refresh() tea.Cmd {
	return m.fetchImages
}

// SetFilter sets the name filter and re-renders the table
func (m *ImagesModel) SetFilter(q string) {
	m.filterQuery = q
	m.updateTable()
}

// Count returns the number of displayed images
func (m *ImagesModel) Count() int {
	return m.filteredCount
}

// GetSelectedImage returns the currently selected image
func (m *ImagesModel) GetSelectedImage() *image.Summary {
	row := m.table.SelectedRow()
	if row == nil || len(m.rows) == 0 {
		return nil
	}
	for i := range m.rows {
		if m.rows[i].repository == row[0] && m.rows[i].tag == row[1] {
			return &m.rows[i].summary
		}
	}
	return nil
}

func (m *ImagesModel) fetchImages() tea.Msg {
	// Listing images can be considerably slower than other Docker list
	// calls (observed 60s+ against a Colima daemon with ~80 images, vs.
	// under a second for containers/volumes/networks), so this gets a
	// longer timeout than the rest of the Docker views.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	images, err := m.client.ListImages(ctx)
	if err != nil {
		return ErrorMsg{Err: err}
	}

	return ImagesMsg{Images: images}
}

func buildImageRows(images []image.Summary) []imageRow {
	var rows []imageRow
	for _, img := range images {
		if len(img.RepoTags) == 0 {
			rows = append(rows, imageRow{summary: img, repository: "<none>", tag: "<none>"})
			continue
		}
		for _, rt := range img.RepoTags {
			repo, tag := rt, "<none>"
			if idx := strings.LastIndex(rt, ":"); idx != -1 {
				repo, tag = rt[:idx], rt[idx+1:]
			}
			rows = append(rows, imageRow{summary: img, repository: repo, tag: tag})
		}
	}
	return rows
}

func (m *ImagesModel) updateTable() {
	q := strings.ToLower(m.filterQuery)
	var rows []table.Row

	for _, r := range m.rows {
		if q != "" && !strings.Contains(strings.ToLower(r.repository), q) {
			continue
		}
		id := r.summary.ID
		id = strings.TrimPrefix(id, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}
		rows = append(rows, table.Row{
			r.repository,
			r.tag,
			id,
			formatBytes(r.summary.Size),
			formatAge(time.Unix(r.summary.Created, 0)),
		})
	}

	m.filteredCount = len(rows)
	m.table.SetRows(rows)
}

// formatBytes renders a byte count as a short human-readable size.
func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// Message types
type ImagesMsg struct {
	Images []image.Summary
}
