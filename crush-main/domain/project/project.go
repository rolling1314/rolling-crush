package project

import (
	"context"
	"database/sql"

	"github.com/rolling1314/rolling-crush/store/postgres"
	"github.com/google/uuid"
)

type Project struct {
	ID               string
	UserID           string
	Name             string
	Description      sql.NullString
	CreatedAt        int64
	UpdatedAt        int64
	ExternalIP       string
	FrontendPort     int32
	WorkspacePath    string
	ContainerName    sql.NullString
	WorkdirPath      sql.NullString
	DbHost           sql.NullString
	DbPort           sql.NullInt32
	DbUser           sql.NullString
	DbPassword       sql.NullString
	DbName           sql.NullString
	BackendPort      sql.NullInt32
	FrontendCommand  sql.NullString
	FrontendLanguage sql.NullString
	BackendCommand   sql.NullString
	BackendLanguage  sql.NullString
}

type Service interface {
	Create(ctx context.Context, userID, name, description, externalIP, workspacePath string, frontendPort int32) (Project, error)
	GetByID(ctx context.Context, id string) (Project, error)
	ListByUser(ctx context.Context, userID string) ([]Project, error)
	Update(ctx context.Context, project Project) (Project, error)
	Delete(ctx context.Context, id string) error
	GetSessions(ctx context.Context, projectID string) ([]postgres.Session, error)
}

type service struct {
	q postgres.Querier
}

func NewService(q postgres.Querier) Service {
	return &service{q: q}
}

func (s *service) Create(ctx context.Context, userID, name, description, externalIP, workspacePath string, frontendPort int32) (Project, error) {
	dbProject, err := s.q.CreateProject(ctx, postgres.CreateProjectParams{
		ID:               uuid.New().String(),
		UserID:           userID,
		Name:             name,
		Description:      sql.NullString{String: description, Valid: description != ""},
		ExternalIP:       externalIP,
		FrontendPort:     frontendPort,
		WorkspacePath:    workspacePath,
		ContainerName:    sql.NullString{Valid: false},
		WorkdirPath:      sql.NullString{Valid: false},
		DbHost:           sql.NullString{Valid: false},
		DbPort:           sql.NullInt32{Valid: false},
		DbUser:           sql.NullString{Valid: false},
		DbPassword:       sql.NullString{Valid: false},
		DbName:           sql.NullString{Valid: false},
		BackendPort:      sql.NullInt32{Valid: false},
		FrontendCommand:  sql.NullString{Valid: false},
		FrontendLanguage: sql.NullString{Valid: false},
		BackendCommand:   sql.NullString{Valid: false},
		BackendLanguage:  sql.NullString{Valid: false},
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
	dbProject, err := s.q.UpdateProject(ctx, postgres.UpdateProjectParams{
		ID:               project.ID,
		Name:             project.Name,
		Description:      project.Description,
		ExternalIP:       project.ExternalIP,
		FrontendPort:     project.FrontendPort,
		WorkspacePath:    project.WorkspacePath,
		ContainerName:    project.ContainerName,
		WorkdirPath:      project.WorkdirPath,
		DbHost:           project.DbHost,
		DbPort:           project.DbPort,
		DbUser:           project.DbUser,
		DbPassword:       project.DbPassword,
		DbName:           project.DbName,
		BackendPort:      project.BackendPort,
		FrontendCommand:  project.FrontendCommand,
		FrontendLanguage: project.FrontendLanguage,
		BackendCommand:   project.BackendCommand,
		BackendLanguage:  project.BackendLanguage,
	})
	if err != nil {
		return Project{}, err
	}
	return s.fromDBItem(dbProject), nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	return s.q.DeleteProject(ctx, id)
}

func (s *service) GetSessions(ctx context.Context, projectID string) ([]postgres.Session, error) {
	return s.q.GetProjectSessions(ctx, sql.NullString{String: projectID, Valid: true})
}

func (s *service) fromDBItem(item postgres.Project) Project {
	return Project{
		ID:               item.ID,
		UserID:           item.UserID,
		Name:             item.Name,
		Description:      item.Description,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		ExternalIP:       item.ExternalIP,
		FrontendPort:     item.FrontendPort,
		WorkspacePath:    item.WorkspacePath,
		ContainerName:    item.ContainerName,
		WorkdirPath:      item.WorkdirPath,
		DbHost:           item.DbHost,
		DbPort:           item.DbPort,
		DbUser:           item.DbUser,
		DbPassword:       item.DbPassword,
		DbName:           item.DbName,
		BackendPort:      item.BackendPort,
		FrontendCommand:  item.FrontendCommand,
		FrontendLanguage: item.FrontendLanguage,
		BackendCommand:   item.BackendCommand,
		BackendLanguage:  item.BackendLanguage,
	}
}
