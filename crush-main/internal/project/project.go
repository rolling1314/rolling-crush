package project

import (
	"context"
	"database/sql"

	"github.com/charmbracelet/crush/internal/db"
	"github.com/google/uuid"
)

type Project struct {
	ID            string
	UserID        string
	Name          string
	Description   sql.NullString
	Host          string
	Port          int32
	WorkspacePath string
	ContainerName sql.NullString
	WorkdirPath   sql.NullString
	CreatedAt     int64
	UpdatedAt     int64
}

type Service interface {
	Create(ctx context.Context, userID, name, description, host, workspacePath string, port int32) (Project, error)
	GetByID(ctx context.Context, id string) (Project, error)
	ListByUser(ctx context.Context, userID string) ([]Project, error)
	Update(ctx context.Context, project Project) (Project, error)
	Delete(ctx context.Context, id string) error
	GetSessions(ctx context.Context, projectID string) ([]db.Session, error)
}

type service struct {
	q db.Querier
}

func NewService(q db.Querier) Service {
	return &service{q: q}
}

func (s *service) Create(ctx context.Context, userID, name, description, host, workspacePath string, port int32) (Project, error) {
	dbProject, err := s.q.CreateProject(ctx, db.CreateProjectParams{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          name,
		Description:   sql.NullString{String: description, Valid: description != ""},
		Host:          host,
		Port:          port,
		WorkspacePath: workspacePath,
		ContainerName: sql.NullString{Valid: false},
		WorkdirPath:   sql.NullString{Valid: false},
	})
	if err != nil {
		return Project{}, err
	}
	return s.fromDBItem(dbProject), nil
}

func (s *service) GetByID(ctx context.Context, id string) (Project, error) {
	dbProject, err := s.q.GetProjectByID(ctx, id)
	if err != nil {
		return Project{}, err
	}
	return s.fromDBItem(dbProject), nil
}

func (s *service) ListByUser(ctx context.Context, userID string) ([]Project, error) {
	dbProjects, err := s.q.ListProjectsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	projects := make([]Project, len(dbProjects))
	for i, dbProject := range dbProjects {
		projects[i] = s.fromDBItem(dbProject)
	}
	return projects, nil
}

func (s *service) Update(ctx context.Context, project Project) (Project, error) {
	dbProject, err := s.q.UpdateProject(ctx, db.UpdateProjectParams{
		ID:            project.ID,
		Name:          project.Name,
		Description:   project.Description,
		Host:          project.Host,
		Port:          project.Port,
		WorkspacePath: project.WorkspacePath,
		ContainerName: project.ContainerName,
		WorkdirPath:   project.WorkdirPath,
	})
	if err != nil {
		return Project{}, err
	}
	return s.fromDBItem(dbProject), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.q.DeleteProject(ctx, id)
}

func (s *service) GetSessions(ctx context.Context, projectID string) ([]db.Session, error) {
	return s.q.GetProjectSessions(ctx, sql.NullString{String: projectID, Valid: true})
}

func (s *service) fromDBItem(item db.Project) Project {
	return Project{
		ID:            item.ID,
		UserID:        item.UserID,
		Name:          item.Name,
		Description:   item.Description,
		Host:          item.Host,
		Port:          item.Port,
		WorkspacePath: item.WorkspacePath,
		ContainerName: item.ContainerName,
		WorkdirPath:   item.WorkdirPath,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

