package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Project struct {
	OwnerID		 int `json:"ownerID"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	StartDate    string   `json:"start_date"`
	EndDate      string   `json:"end_date"`
	Participants []int    `json:"participants"`
}


type Claims struct {
    UserID int `json:"id"`
	Username string `json:"username"`
	Email string `json:"email"`
	UserRole string `json:"role"`
	jwt.RegisteredClaims
}

type loginRequest struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

func (app *application) getProjectsHandler(c *gin.Context){
	projects, err := app.database_handler.GetProjects()
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить проекты"})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (app *application)createProjectHandler(c *gin.Context) {
	var project Project
	if err := c.ShouldBindJSON(&project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Вставка проекта в таблицу
	var projectID int
	log.Printf("dsd", project.OwnerID)
	//ownerID, _ := strconv.Atoi(project.OwnerID)
	
	projectID ,err := app.database_handler.CreateProject(project.Title, project.Description,project.StartDate,project.EndDate, project.OwnerID)
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
		err := app.database_handler.AddUsersProjects( projectID, participantID)
		fmt.Print("ds")
        fmt.Println(participantID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to add participant %d", participantID)})
			return
		}
	}
	

	c.JSON(http.StatusOK, gin.H{"message": "Project created successfully"})
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

    expirationTime := time.Now().Add(1*time.Hour)
    claims := &Claims{
		UserID: user.UsrID,
        Username: req.Username,
		Email: user.UsrEmail,
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

func (app *application) getUsersHandler(c *gin.Context){
    users, err := app.database_handler.GetUsers()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить пользователей"})
        return
    }
    c.JSON(http.StatusOK, users)
}



