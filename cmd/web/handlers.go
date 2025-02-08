package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Project struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	Status 		 string `json:"status"`
	Participants []int  `json:"participants"`
	OwnerID      int    `json:"ownerID"`
}

type Task struct {
	ID int `json:"id"`
    TskPrjId int `json:"tsk_prj_id"`
    Title string `json:"title"`
    Description string `json:"description"`
    Priority string `json:"priority"`
    Status string `json:"status"`
    AssigneId string `json:"tsk_assigne_id"`
}

type Claims struct {
	UserID   int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	UserRole string `json:"role"`
	jwt.RegisteredClaims
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (app *application) getProjectsHandler(c *gin.Context) {
	projects, err := app.database_handler.GetProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить проекты"})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (app *application) updateProjectHandler(c *gin.Context) {
	var Project struct {
		ID          int `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
	}
	if err := c.ShouldBindJSON(&Project);err != nil {
		fmt.Printf("Error updating project: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update project", "details": err.Error()})
		return
	}

	err := app.database_handler.UpdateProject(Project.Title, Project.Description, Project.StartDate, Project.EndDate, Project.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Filed to update project"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Project inserted successfully"})
}

func (app *application) deleteProjectHandler(c *gin.Context) {
    prj_id_str := c.Param("id")
    prj_id, _ := strconv.Atoi(prj_id_str)
    err := app.database_handler.DeleteProject(prj_id)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка удаления проекта!"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Проект успешно удален"})
}

func (app *application) getProjectHandler(c *gin.Context) {
    prj_id_str := c.Param("id")
    prj_id, _ := strconv.Atoi(prj_id_str)
    project, err := app.database_handler.GetProject(prj_id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить проект"})
        return
    }
    c.JSON(http.StatusOK, project)
}

func (app *application) loginHandler(c *gin.Context) {
	var req loginRequest

	// Привязка данных JSON к структуре
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	log.Printf("Login attempt with username: %s", req.Username)

	// Проверка пользователя в базе данных
	user, err := app.database_handler.IsValidUser(req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	log.Printf("Login attempt with username: %s", user.UsrRole)

	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		UserID:   user.UsrID,
		Username: req.Username,
		Email:    user.UsrEmail,
		UserRole: user.UsrRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte("S3krETB2LUY4dm1WME5YYQ=="))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create token"})
		return
	}
	// Успешный вход
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "user": user, "token": tokenString})
}

func (app *application) getUsersHandler(c *gin.Context) {
	users, err := app.database_handler.GetUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить пользователей"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (app *application) createTaskHandler(c *gin.Context){
	var task Task
	fmt.Print("fdf")
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}
	fmt.Print("fd1f")

	task.Status = "Новая"

	err := app.database_handler.CreateTask(task.TskPrjId, task.Title, task.Description, task.Priority, task.Status)
	fmt.Print("task")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		fmt.Print(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task created successfully"})
}

func (app *application) createProjectHandler(c *gin.Context) {
	var project Project
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	var projectID int
	prj_status := "Активный"
	projectID, err := app.database_handler.CreateProject(project.Title, project.Description, project.StartDate, project.EndDate, prj_status, project.OwnerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project"})
		return
	}

	for _, participant := range project.Participants {
		fmt.Print("dd")
		fmt.Println(participant)
	}
	// Добавление участников в проект
	for _, participantID := range project.Participants {
		err := app.database_handler.AddUsersProjects(projectID, participantID)
		fmt.Print("ds")
		fmt.Println(participantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add participant %d", participantID)})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Project created successfully"})
}
