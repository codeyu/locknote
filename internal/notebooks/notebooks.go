// NaishoMemo — 基于 LockNote（https://github.com/JackyZhang8/locknote，MIT 许可）修改。
package notebooks

import (
	"naishomemo/internal/database"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	db *database.DB
}

type Notebook struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sortOrder"`
	Pinned    bool   `json:"pinned"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

func NewService(db *database.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(name, icon string) (*Notebook, error) {
	if icon == "" {
		icon = "📓"
	}

	sortOrder, err := s.db.GetNextNotebookSortOrder()
	if err != nil {
		sortOrder = 0
	}

	now := time.Now()
	notebook := &database.Notebook{
		ID:        uuid.New().String(),
		Name:      name,
		Icon:      icon,
		SortOrder: sortOrder,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.db.CreateNotebook(notebook); err != nil {
		return nil, err
	}

	return &Notebook{
		ID:        notebook.ID,
		Name:      notebook.Name,
		Icon:      notebook.Icon,
		SortOrder: notebook.SortOrder,
		Pinned:    notebook.Pinned,
		CreatedAt: formatTime(notebook.CreatedAt),
		UpdatedAt: formatTime(notebook.UpdatedAt),
	}, nil
}

func (s *Service) Update(id, name, icon string) (*Notebook, error) {
	notebook, err := s.db.GetNotebook(id)
	if err != nil {
		return nil, err
	}

	notebook.Name = name
	if icon != "" {
		notebook.Icon = icon
	}
	notebook.UpdatedAt = time.Now()

	if err := s.db.UpdateNotebook(notebook); err != nil {
		return nil, err
	}

	return &Notebook{
		ID:        notebook.ID,
		Name:      notebook.Name,
		Icon:      notebook.Icon,
		SortOrder: notebook.SortOrder,
		Pinned:    notebook.Pinned,
		CreatedAt: formatTime(notebook.CreatedAt),
		UpdatedAt: formatTime(notebook.UpdatedAt),
	}, nil
}

func (s *Service) Delete(id string) error {
	return s.db.DeleteNotebook(id)
}

func (s *Service) List() ([]*Notebook, error) {
	dbNotebooks, err := s.db.ListNotebooks()
	if err != nil {
		return nil, err
	}

	notebooks := make([]*Notebook, len(dbNotebooks))
	for i, n := range dbNotebooks {
		notebooks[i] = &Notebook{
			ID:        n.ID,
			Name:      n.Name,
			Icon:      n.Icon,
			SortOrder: n.SortOrder,
			Pinned:    n.Pinned,
			CreatedAt: formatTime(n.CreatedAt),
			UpdatedAt: formatTime(n.UpdatedAt),
		}
	}

	return notebooks, nil
}

func (s *Service) SetPinned(id string, pinned bool) error {
	return s.db.SetNotebookPinned(id, pinned)
}

func (s *Service) UpdateSortOrder(id string, sortOrder int) error {
	return s.db.UpdateNotebookSortOrder(id, sortOrder)
}

func (s *Service) ReorderNotebooks(ids []string) error {
	return s.db.ReorderNotebooks(ids)
}
