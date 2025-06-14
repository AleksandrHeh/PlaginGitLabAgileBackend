package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"golangify.com/plaginagile/pkg/models/pgsql"
)

type application struct {
	errorLog     *log.Logger
	infoLog      *log.Logger
	oauthHandler *OAuthHandler
	models       *pgsql.PullIncludes
	db           *pgxpool.Pool
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP network address")
	dsn := flag.String("dsn", "postgres://postgres:BUGLb048@localhost:5432/GitLabAgile", "Название PgSQL источника данных")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	db, err := openDB(*dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &application{
		errorLog: errorLog,
		infoLog:  infoLog,
		models:   &pgsql.PullIncludes{DB: db},
		db:       db,
	}

	// Настройки OAuth для GitLab
	// Важно: эти настройки должны соответствовать настройкам в GitLab
	// 1. Войдите в GitLab как администратор
	// 2. Перейдите в Admin Area -> Applications
	// 3. Создайте новое приложение с настройками:
	//    - Name: PlaginAgile
	//    - Redirect URI: http://localhost:8080/oauth/callback
	//    - Scopes: api, read_api
	//    - Trusted: Yes
	//    - Confidential: Yes
	oauthHandler := &OAuthHandler{
		clientID:      "9feaef0a6c56d9765985fa701db9c1dfd332389c865c44baa94b916d6b080712", // Замените на новый Application ID
		clientSecret:  "gloas-22f4423835a6b9d937ec329fb9496e4e6f1bef96205d7a03bcb9bec59b4779ca", // Замените на новый Secret
		redirectURI:   "http://localhost:8080/oauth/callback",
		gitlabBaseURL: "http://gitlab.example.com",
		app:           app,
	}

	app.oauthHandler = oauthHandler

	router := app.routes()

	infoLog.Printf("Start server on %s", *addr)
	err = router.Run(*addr)
	if err != nil {
		log.Fatal(err)
	}
}

func openDB(dsn string) (*pgxpool.Pool, error) {
	bd, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}

	err = bd.Ping(context.Background())
	if err != nil {
		bd.Close()
		return nil, err
	}
	return bd, nil
}

func (app *application) routes() *gin.Engine {
	router := gin.Default()

	// Настройка CORS
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Маршруты для GitLab OAuth
	router.GET("/oauth/callback", app.oauthHandler.GitLabCallbackHandler)
	router.GET("/api/gitlab/auth", app.oauthHandler.GitLabAuthHandler)

	// Остальные маршруты GitLab
	gitlab := router.Group("/api/gitlab")
	{
		gitlab.GET("/projects", app.oauthHandler.GitLabProjectsHandler)
		gitlab.GET("/projects/:id", app.oauthHandler.GitLabProjectHandler)
		gitlab.GET("/projects/:id/issues", app.oauthHandler.GitLabProjectIssuesHandler)
		gitlab.GET("/projects/:id/members", app.oauthHandler.GitLabProjectMembersHandler)
		gitlab.GET("/members", app.oauthHandler.GitLabMembersHandler)
		gitlab.POST("/projects/:id/issues", app.oauthHandler.CreateGitLabIssue)
		gitlab.PUT("/users/:id/role", app.oauthHandler.UpdateUserRoleHandler)
		gitlab.POST("/projects", app.oauthHandler.CreateGitLabProject)
		gitlab.PUT("/projects/:id", app.oauthHandler.UpdateGitLabProject)
		gitlab.DELETE("/projects/:id", app.oauthHandler.DeleteGitLabProject)
		gitlab.DELETE("/projects/:id/issues/:issueId", app.oauthHandler.DeleteGitLabIssue)
	}

	// Добавляем маршрут для обработки callback'а
	// User routes
	router.GET("/api/users", app.oauthHandler.GetUsersHandler)

	// Маршруты для проектов
	router.POST("/api/projects", app.oauthHandler.SaveProjectMetadata)

	// Маршруты для спринтов
	sprints := router.Group("/api/projects/:id/sprints")
	{
		sprints.GET("", app.getSprints)
		sprints.POST("", app.createSprint)
		sprints.GET("/:sprintId", app.getSprint)
		sprints.PUT("/:sprintId", app.updateSprint)
		sprints.DELETE("/:sprintId", app.deleteSprint)
		sprints.POST("/:sprintId/complete", app.completeSprint)
		sprints.GET("/:sprintId/issues", app.getSprintIssues)
		sprints.POST("/:sprintId/issues", app.addIssueToSprint)
		sprints.GET("/:sprintId/issues/:taskId", app.getSprintIssue)
		sprints.PUT("/:sprintId/issues/:taskId/assignee", app.updateIssueAssignee)
		sprints.PUT("/:sprintId/issues/:taskId/status", app.updateIssueStatus)
		sprints.DELETE("/:sprintId/issues/:taskId", app.deleteSprintIssue)
	}

	// Маршрут для GitLab вебхуков
	router.POST("/api/webhooks/gitlab", app.HandleGitLabWebhook)

	return router
}