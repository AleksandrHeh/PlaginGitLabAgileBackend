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

	oauthHandler := &OAuthHandler{
		clientID:      "9feaef0a6c56d9765985fa701db9c1dfd332389c865c44baa94b916d6b080712",
		clientSecret:  "gloas-22f4423835a6b9d937ec329fb9496e4e6f1bef96205d7a03bcb9bec59b4779ca",
		redirectURI:   "http://localhost:8080/oauth/callback",
		gitlabBaseURL: "http://localhost",
	}

	app := &application{
		errorLog:     errorLog,
		infoLog:      infoLog,
		oauthHandler: oauthHandler,
		models:       &pgsql.PullIncludes{DB: db},
	}

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
	router.GET("/api/gitlab/auth", app.oauthHandler.GitLabAuthHandler)
	router.GET("/api/gitlab/callback", app.oauthHandler.GitLabCallbackHandler)
	router.GET("/api/gitlab/projects", app.oauthHandler.GitLabProjectsHandler)
	router.GET("/api/gitlab/projects/:id", app.oauthHandler.GitLabProjectHandler)
	router.GET("/api/gitlab/projects/:id/issues", app.oauthHandler.GitLabProjectIssuesHandler)
	router.GET("/api/users", app.oauthHandler.GitLabMembersHandler)
	router.POST("/api/gitlab/projects/:id/issues", app.oauthHandler.CreateGitLabIssue)

	// Маршруты для проектов
	router.POST("/api/projects", app.oauthHandler.SaveProjectMetadata)
	router.GET("/api/projects/:id/sprints", app.getSprints)
	router.POST("/api/sprints", app.createSprint)
	router.POST("/api/sprints/issues", app.addIssueToSprint)

	return router
}
