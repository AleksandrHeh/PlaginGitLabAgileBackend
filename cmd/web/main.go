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
	database_handler *pgsql.PullIncludes
}

func main() {
	addr := flag.String("addr", ":4000", "HTTP network address")
	dsn := flag.String("dsn", "postgres://postgres:BUGLb048@localhost:5432/Agile", "Название PgSQL источника данных")
	flag.Parse()

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	//errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	db, err := openDB(*dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := &application{
		database_handler: &pgsql.PullIncludes{DB: db},
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

	// маршруты
	router.POST("/api/login", app.loginHandler)
	router.GET("/api/users", app.getUsersHandler)
	router.POST("/api/createProject", app.createProjectHandler)
	router.GET("/api/viewProjects", app.getProjectsHandler)
	router.PUT("/api/updateProject", app.updateProjectHandler)
	router.DELETE("/api/deleteProject/:id", app.deleteProjectHandler)

	return router
}
