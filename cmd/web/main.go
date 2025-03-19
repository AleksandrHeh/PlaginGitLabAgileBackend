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
    databaseHandler *pgsql.PullIncludes
    oauthHandler    *OAuthHandler // Новый обработчик OAuth
}

func main() {
    addr := flag.String("addr", ":4000", "HTTP network address")
    dsn := flag.String("dsn", "postgres://postgres:BUGLb048@localhost:5432/Agile", "Название PgSQL источника данных")
    flag.Parse()

    infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)

    db, err := openDB(*dsn)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

	oauthHandler := &OAuthHandler{
		clientID:     "9feaef0a6c56d9765985fa701db9c1dfd332389c865c44baa94b916d6b080712",
		clientSecret: "gloas-22f4423835a6b9d937ec329fb9496e4e6f1bef96205d7a03bcb9bec59b4779ca",
		redirectURI:  "http://localhost:8080/oauth/callback", // Укажите адрес вашего фронтенда
		gitlabBaseURL: "http://localhost",
	}

    app := &application{
        databaseHandler: &pgsql.PullIncludes{DB: db},
        oauthHandler:    oauthHandler,
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
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204) // Если preflight-запрос, просто вернуть 204
            return
        }
        c.Next()
    })

    // Маршруты для OAuth
    router.GET("/api/gitlab/auth", app.oauthHandler.GitLabAuthHandler)
    router.GET("/api/gitlab/callback", app.oauthHandler.GitLabCallbackHandler)
	router.GET("/api/gitlab/projects", app.oauthHandler.GitLabProjectsHandler)
    router.GET("api/gitlab/projects/:id", app.oauthHandler.GitLabProjectHandler)
    router.GET("/api/gitlab/projects/:id/issues", app.oauthHandler.GitLabProjectIssuesHandler)

    return router
}