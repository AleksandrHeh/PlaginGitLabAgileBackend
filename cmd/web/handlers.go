package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golangify.com/plaginagile/pkg/models"
)

type GitLabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
}

type OAuthHandler struct {
	clientID      string
	clientSecret  string
	redirectURI   string
	gitlabBaseURL string // Базовый URL для локального GitLab
	app           *application
}

func (h *OAuthHandler) GitLabAuthHandler(c *gin.Context) {
	authURL := fmt.Sprintf(
		"%s/oauth/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=api+read_api",
		h.gitlabBaseURL, h.clientID, url.QueryEscape(h.redirectURI),
	)
	fmt.Println("Redirect URI:", authURL)
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

func (h *OAuthHandler) GitLabCallbackHandler(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code is missing"})
		return
	}

	tokenURL := fmt.Sprintf("%s/oauth/token", h.gitlabBaseURL)
	formData := url.Values{
		"client_id":     {h.clientID},
		"client_secret": {h.clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {h.redirectURI},
	}

	resp, err := http.PostForm(tokenURL, formData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange code for token"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse token response"})
		return
	}

	user, err := h.authenticateWithGitLab(tokenResponse.AccessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to authenticate with GitLab"})
		return
	}

	// Возвращаем данные пользователя и URL для перенаправления
	c.JSON(http.StatusOK, gin.H{
		"user":        user,
		"token":       tokenResponse.AccessToken,
		"redirectURL": "/home", // Указываем, куда перенаправить пользователя
	})
}
func (h *OAuthHandler) authenticateWithGitLab(token string) (*GitLabUser, error) {
	url := fmt.Sprintf("%s/api/v4/user", h.gitlabBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка аутентификации: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var user GitLabUser
	err = json.Unmarshal(body, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (h *OAuthHandler) GitLabProjectsHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	url := fmt.Sprintf("%s/api/v4/projects", h.gitlabBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка запроса к GitLab:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Ошибка чтения ответа:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	// Проверяем, является ли ответ ошибкой
	var errorResponse map[string]interface{}
	if err := json.Unmarshal(body, &errorResponse); err == nil {
		if _, exists := errorResponse["error"]; exists {
			fmt.Println("Ошибка от GitLab:", errorResponse)
			c.JSON(http.StatusForbidden, gin.H{"error": errorResponse["error"]})
			return
		}
	}

	// Парсим данные как массив проектов
	var projects []map[string]interface{}
	if err := json.Unmarshal(body, &projects); err != nil {
		fmt.Println("Ошибка парсинга данных:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных"})
		return
	}

	c.JSON(http.StatusOK, projects)
}

func (h *OAuthHandler) GitLabProjectHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	projectID := c.Param("id") // Получаем ID проекта из URL
	url := fmt.Sprintf("%s/api/v4/projects/%s", h.gitlabBaseURL, projectID)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	var project map[string]interface{}
	if err := json.Unmarshal(body, &project); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных"})
		return
	}

	c.JSON(http.StatusOK, project)
}

func (h *OAuthHandler) GitLabProjectIssuesHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	projectID := c.Param("id") // Получаем ID проекта из URL
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues", h.gitlabBaseURL, projectID)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	var issues []map[string]interface{}
	if err := json.Unmarshal(body, &issues); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных"})
		return
	}

	c.JSON(http.StatusOK, issues)
}

func (h *OAuthHandler) GitLabMembersHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	// Получаем список пользователей
	usersURL := fmt.Sprintf("%s/api/v4/users", h.gitlabBaseURL)
	req, err := http.NewRequest("GET", usersURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка запроса к GitLab:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Ошибка чтения ответа:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		fmt.Println("Ошибка парсинга данных пользователей:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных пользователей"})
		return
	}

	// Получаем роли для каждого пользователя из нашей БД
	var members []map[string]interface{}
	for _, user := range users {
		userID := int(user["id"].(float64))
		
		// Получаем настройки пользователя из нашей БД
		var settings struct {
			Role string `json:"role"`
		}
		
		err := h.app.db.QueryRow(context.Background(), `
			SELECT us_role 
			FROM user_settings 
			WHERE us_user_id = $1
		`, userID).Scan(&settings.Role)
		
		// Если настройки не найдены, используем значение по умолчанию
		if err != nil {
			if err == sql.ErrNoRows {
				settings.Role = "developer"
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения настроек пользователя"})
				return
			}
		}
		
		// Объединяем данные из GitLab и нашей БД
		memberWithSettings := map[string]interface{}{
			"id":         user["id"],
			"name":       user["name"],
			"email":      user["email"],
			"avatar_url": user["avatar_url"],
			"username":   user["username"],
			"state":      user["state"],
			"userSettings": map[string]interface{}{
				"us_role": settings.Role,
			},
		}
		
		members = append(members, memberWithSettings)
	}

	c.JSON(http.StatusOK, members)
}

func (h *OAuthHandler) CreateGitLabIssue(c *gin.Context) {
	projectId := c.Param("id")            // ID проекта из URL
	token := c.GetHeader("Authorization") // Токен авторизации

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	// Получаем данные из тела запроса
	var issueData struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Labels      string `json:"labels"`
	}

	if err := c.ShouldBindJSON(&issueData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные запроса"})
		return
	}

	// Формируем URL для GitLab API
	url := fmt.Sprintf("%s/api/v4/projects/%s/issues", h.gitlabBaseURL, projectId)

	// Создаем запрос
	reqBody, err := json.Marshal(map[string]string{
		"title":       issueData.Title,
		"description": issueData.Description,
		"labels":      issueData.Labels,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	// Отправляем запрос к GitLab API
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	// Обрабатываем ответ
	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		return
	}

	var createdIssue map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&createdIssue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга ответа"})
		return
	}

	c.JSON(http.StatusCreated, createdIssue)
}

func (h *OAuthHandler) SaveProjectMetadata(c *gin.Context) {
	var projectData struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		StartDate    string `json:"start_date"`
		EndDate      string `json:"end_date"`
		Participants []int  `json:"participants"`
	}

	if err := c.ShouldBindJSON(&projectData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные запроса"})
		return
	}

	// Шаг 1: Создаем проект в GitLab
	gitlabURL := fmt.Sprintf("%s/api/v4/projects", h.gitlabBaseURL)
	gitlabResponse, err := http.PostForm(gitlabURL, url.Values{
		"name":        {projectData.Title},
		"description": {projectData.Description},
		"visibility":  {"private"},
	})

	if err != nil || gitlabResponse.StatusCode != http.StatusCreated {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании проекта в GitLab"})
		return
	}

	defer gitlabResponse.Body.Close()

	// Парсим ответ от GitLab
	var gitlabProject map[string]interface{}
	if err := json.NewDecoder(gitlabResponse.Body).Decode(&gitlabProject); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга ответа от GitLab"})
		return
	}

	gitlabProjectID := gitlabProject["id"].(float64) // ID проекта в GitLab

	// Шаг 2: Сохраняем метаданные в вашей базе данных
	// Здесь добавьте логику сохранения данных в вашу базу данных
	// Например, используйте ORM или SQL-запросы

	c.JSON(http.StatusCreated, gin.H{
		"message": "Проект успешно создан",
		"project": gin.H{
			"gitlab_project_id": gitlabProjectID,
			"title":             projectData.Title,
			"description":       projectData.Description,
			"start_date":        projectData.StartDate,
			"end_date":          projectData.EndDate,
			"participants":      projectData.Participants,
		},
	})
}

type CreateSprintRequest struct {
	Title     string    `json:"title"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Goals     string    `json:"goals"`
	ProjectID int       `json:"project_id"`
}

func (app *application) createSprint(c *gin.Context) {
	var req CreateSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if req.Title == "" || req.ProjectID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать название спринта и ID проекта"})
		return
	}

	sprintID, err := app.models.CreateSprint(req.Title, req.StartDate, req.EndDate, req.Goals, req.ProjectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка при создании спринта: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"sprint_id": sprintID,
	})
}

// getSprints получает список спринтов проекта
func (app *application) getSprints(c *gin.Context) {
	projectID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID проекта"})
		return
	}

	sprints, err := app.models.GetSprints(projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sprints)
}

type AddIssueToSprintRequest struct {
	SprintID    int    `json:"sprint_id"`
	IssueID     int    `json:"issue_id"`
	StoryPoints int    `json:"story_points"`
	Priority    string `json:"priority"`
	NameIssue   string `json:"name_issue"`
	DescriptionIssue string `json:"description_issue"`
}

func (app *application) addIssueToSprint(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		app.errorLog.Printf("Неверный формат ID спринта: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
		return
	}

	// Проверяем существование спринта
	_, err = app.models.GetSprint(sprintID)
	if err != nil {
		app.errorLog.Printf("Ошибка получения спринта: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Спринт не найден"})
		return
	}

	var req AddIssueToSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		app.errorLog.Printf("Ошибка парсинга JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	app.infoLog.Printf("Получен запрос на добавление задачи в спринт: sprintID=%d, issueID=%d", 
		sprintID, req.IssueID)

	if req.IssueID == 0 {
		app.errorLog.Printf("Отсутствует ID задачи в запросе")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать ID задачи"})
		return
	}

	// Проверяем, не добавлена ли уже задача в спринт
	existingIssue, err := app.models.GetSprintIssue(sprintID, req.IssueID)
	if err == nil && existingIssue != nil {
		app.errorLog.Printf("Задача %d уже добавлена в спринт %d", req.IssueID, sprintID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Задача уже добавлена в этот спринт"})
		return
	}

	err = app.models.AddIssueToSprint(sprintID, req.IssueID, req.StoryPoints, req.Priority, req.NameIssue, req.DescriptionIssue)
	if err != nil {
		app.errorLog.Printf("Ошибка добавления задачи в спринт: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	app.infoLog.Printf("Задача %d успешно добавлена в спринт %d", req.IssueID, sprintID)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// getSprint получает данные конкретного спринта
func (app *application) getSprint(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID спринта"})
		return
	}

	sprint, err := app.models.GetSprint(sprintID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sprint)
}

// getSprintIssues получает задачи спринта и синхронизирует их статусы с GitLab
func (app *application) getSprintIssues(c *gin.Context) {
    sprintID, err := strconv.Atoi(c.Param("sprintId"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
        return
    }

    // Получаем токен из заголовка
    token := c.GetHeader("Authorization")
    if token == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
        return
    }

    // Получаем ID проекта из параметров запроса
    projectID, err := strconv.Atoi(c.Param("id"))
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID проекта"})
        return
    }

    // Синхронизируем статусы задач с GitLab
    if err := app.syncAllIssuesWithGitLab(sprintID, projectID, token); err != nil {
        app.errorLog.Printf("Ошибка синхронизации задач с GitLab: %v", err)
        // Продолжаем выполнение даже при ошибке синхронизации
    }

    // Получаем обновленные задачи
    issues, err := app.models.GetSprintIssues(sprintID)
    if err != nil {
        if err == models.ErrNoRecord {
            c.JSON(http.StatusNotFound, gin.H{"error": "Спринт или задачи не найдены"})
            return
        }
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка при получении задач: %v", err)})
        return
    }

    // Добавляем информацию о статусе для каждой задачи
    for i := range issues {
        // Если статус не установлен, устанавливаем "К выполнению"
        if issues[i].Status == "" {
            issues[i].Status = "К выполнению"
        }
    }

    c.JSON(http.StatusOK, issues)
}

type UpdateIssueAssigneeRequest struct {
	IssueID    int `json:"issue_id"`
	AssigneeID int `json:"assignee_id"`
}

func (app *application) updateIssueAssignee(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		app.errorLog.Printf("Неверный формат ID спринта: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
		return
	}

	var req UpdateIssueAssigneeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		app.errorLog.Printf("Ошибка парсинга JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	app.infoLog.Printf("Получен запрос на обновление участника задачи: sprintID=%d, issueID=%d, assigneeID=%d", 
		sprintID, req.IssueID, req.AssigneeID)

	if req.IssueID == 0 {
		app.errorLog.Printf("Отсутствует ID задачи в запросе")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать ID задачи"})
		return
	}

	err = app.models.UpdateSprintIssueAssignee(sprintID, req.IssueID, req.AssigneeID)
	if err != nil {
		app.errorLog.Printf("Ошибка обновления участника задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	app.infoLog.Printf("Успешно обновлен участник задачи: sprintID=%d, issueID=%d, assigneeID=%d",
		sprintID, req.IssueID, req.AssigneeID)

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// GitLabWebhookRequest представляет структуру вебхука от GitLab
type GitLabWebhookRequest struct {
	ObjectKind string `json:"object_kind"`
	EventName  string `json:"event_name"`
	Project    struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Commits []struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		Title     string    `json:"title"`
		Timestamp string    `json:"timestamp"`
		Author    struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
	Repository struct {
		Name        string `json:"name"`
		URL         string `json:"url"`
		Description string `json:"description"`
	} `json:"repository"`
	// Добавляем поля для Merge Request
	ObjectAttributes struct {
		ID          int       `json:"id"`
		IID         int       `json:"iid"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		State       string    `json:"state"`
		CreatedAt   string    `json:"created_at"`
		UpdatedAt   string    `json:"updated_at"`
		SourceBranch string   `json:"source_branch"`
		TargetBranch string   `json:"target_branch"`
		LastCommit struct {
			ID string `json:"id"`
		} `json:"last_commit"`
	} `json:"object_attributes"`
}

// HandleGitLabWebhook обрабатывает вебхуки от GitLab
func (app *application) HandleGitLabWebhook(c *gin.Context) {
	eventType := c.Request.Header.Get("X-Gitlab-Event")
	app.infoLog.Printf("Получен вебхук от GitLab: %s", eventType)
	app.infoLog.Printf("Заголовки запроса: %v", c.Request.Header)

	// Читаем тело запроса для логирования
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		app.errorLog.Printf("Ошибка чтения тела запроса: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ошибка чтения тела запроса"})
		return
	}
	// Восстанавливаем тело запроса для дальнейшей обработки
	c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	
	app.infoLog.Printf("Тело вебхука: %s", string(body))

	var webhook GitLabWebhookRequest
	if err := c.ShouldBindJSON(&webhook); err != nil {
		app.errorLog.Printf("Ошибка парсинга вебхука: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных вебхука"})
		return
	}

	app.infoLog.Printf("Тип события: %s, Project ID: %d, Project Name: %s", 
		webhook.EventName, webhook.Project.ID, webhook.Project.Name)
	app.infoLog.Printf("ObjectKind: %s, State: %s", 
		webhook.ObjectKind, webhook.ObjectAttributes.State)

	switch webhook.ObjectKind {
	case "push":
		if err := app.handleGitLabPush(webhook); err != nil {
			app.errorLog.Printf("Ошибка обработки push события: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	case "merge_request":
		app.infoLog.Printf("Обработка merge request события: %s", webhook.ObjectAttributes.State)
		if err := app.handleGitLabMergeRequest(webhook); err != nil {
			app.errorLog.Printf("Ошибка обработки merge request события: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	default:
		app.infoLog.Printf("Получено событие: %s", webhook.ObjectKind)
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// extractIssueIDFromCommitMessage извлекает ID задачи из сообщения коммита
func extractIssueIDFromCommitMessage(message string) int {
	// Разбиваем сообщение на строки
	lines := strings.Split(message, "\n")
	
	// Проверяем каждую строку на наличие ссылки на задачу
	for _, line := range lines {
		// Убираем лишние пробелы
		line = strings.TrimSpace(line)
		
		// Пробуем разные форматы
		var issueID int
		
		// Формат "Fix #123"
		if _, err := fmt.Sscanf(line, "Fix #%d", &issueID); err == nil {
			return issueID
		}
		
		// Формат "Closes #123"
		if _, err := fmt.Sscanf(line, "Closes #%d", &issueID); err == nil {
			return issueID
		}
		
		// Формат "#123"
		if _, err := fmt.Sscanf(line, "#%d", &issueID); err == nil {
			return issueID
		}
		
		// Формат "See merge request ... !123"
		if strings.Contains(line, "See merge request") {
			if _, err := fmt.Sscanf(line, "See merge request %s!%d", nil, &issueID); err == nil {
				return issueID
			}
		}
	}
	
	return 0
}

// handleGitLabPush обрабатывает события push (коммиты)
func (app *application) handleGitLabPush(webhook GitLabWebhookRequest) error {
	if len(webhook.Commits) == 0 {
		app.infoLog.Printf("Получен push без коммитов")
		return nil
	}

	for _, commit := range webhook.Commits {
		app.infoLog.Printf("Обработка коммита: %s, сообщение: %s", commit.ID, commit.Message)
		
		// Извлекаем номер задачи из сообщения коммита
		issueID := extractIssueIDFromCommitMessage(commit.Message)
		if issueID == 0 {
			app.infoLog.Printf("Коммит не содержит ссылки на задачу: %s", commit.Message)
			continue
		}

		app.infoLog.Printf("Найдена ссылка на задачу #%d в коммите", issueID)

		// Получаем спринт, в котором находится задача
		sprintID, err := app.models.GetSprintIDByIssueID(issueID)
		if err != nil {
			app.errorLog.Printf("Ошибка получения спринта для задачи %d: %v", issueID, err)
			continue
		}

		app.infoLog.Printf("Обновление статуса задачи %d в спринте %d", issueID, sprintID)

		// Парсим время коммита из строки ISO 8601
		commitTime, err := time.Parse(time.RFC3339, commit.Timestamp)
		if err != nil {
			app.errorLog.Printf("Ошибка парсинга времени коммита: %v (время: %s)", err, commit.Timestamp)
			commitTime = time.Now() // Используем текущее время как запасной вариант
		}

		// Обновляем статус задачи
		err = app.models.UpdateSprintIssueStatus(
			sprintID,
			issueID,
			"На проверке",
			&commitTime,
			nil,
			"main", // Используем main как ветку по умолчанию
			nil,
		)
		if err != nil {
			app.errorLog.Printf("Ошибка обновления статуса задачи: %v", err)
			continue
		}

		app.infoLog.Printf("Статус задачи %d успешно обновлен", issueID)
	}

	return nil
}

// extractIssueIDFromMergeRequest извлекает ID задачи из названия или описания мердж-реквеста
func extractIssueIDFromMergeRequest(title, description string) int {
	// Разбиваем описание на строки
	lines := strings.Split(description, "\n")
	
	// Проверяем каждую строку на наличие ключевых слов
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Пробуем разные форматы с разными ключевыми словами
		var issueID int
		
		// Форматы: "Closes #123", "Fixes #123", "Resolves #123"
		keywords := []string{"Closes", "Fixes", "Resolves"}
		for _, keyword := range keywords {
			pattern := fmt.Sprintf("%s #%%d", keyword)
			if _, err := fmt.Sscanf(line, pattern, &issueID); err == nil {
				return issueID
			}
		}
		
		// Формат "Fix #123" в названии
		if _, err := fmt.Sscanf(title, "Fix #%d", &issueID); err == nil {
			return issueID
		}
		
		// Формат "#123" в названии или описании
		if _, err := fmt.Sscanf(line, "#%d", &issueID); err == nil {
			return issueID
		}
	}
	
	// Если в описании не нашли, проверяем название
	var issueID int
	if _, err := fmt.Sscanf(title, "#%d", &issueID); err == nil {
		return issueID
	}
	
	return 0
}

// handleGitLabMergeRequest обрабатывает события мердж-реквеста
func (app *application) handleGitLabMergeRequest(webhook GitLabWebhookRequest) error {
	app.infoLog.Printf("Обработка мердж-реквеста #%d: %s (состояние: %s)", 
		webhook.ObjectAttributes.IID, 
		webhook.ObjectAttributes.Title,
		webhook.ObjectAttributes.State)
	
	app.infoLog.Printf("Описание мердж-реквеста: %s", webhook.ObjectAttributes.Description)

	// Извлекаем номер задачи из названия или описания мердж-реквеста
	issueID := extractIssueIDFromMergeRequest(webhook.ObjectAttributes.Title, webhook.ObjectAttributes.Description)
	if issueID == 0 {
		app.infoLog.Printf("Мердж-реквест не содержит ссылки на задачу: %s", webhook.ObjectAttributes.Title)
		return nil
	}

	app.infoLog.Printf("Найдена ссылка на задачу #%d в мердж-реквесте", issueID)

	// Получаем спринт, в котором находится задача
	sprintID, err := app.models.GetSprintIDByIssueID(issueID)
	if err != nil {
		app.errorLog.Printf("Ошибка получения спринта для задачи %d: %v", issueID, err)
		return err
	}

	app.infoLog.Printf("Обновление статуса задачи %d в спринте %d (состояние MR: %s)", 
		issueID, sprintID, webhook.ObjectAttributes.State)

	// Проверяем состояние мердж-реквеста
	switch webhook.ObjectAttributes.State {
	case "merged":
		// Парсим время обновления из строки ISO 8601
		updatedAt, err := time.Parse(time.RFC3339, webhook.ObjectAttributes.UpdatedAt)
		if err != nil {
			app.errorLog.Printf("Ошибка парсинга времени обновления: %v (время: %s)", err, webhook.ObjectAttributes.UpdatedAt)
			updatedAt = time.Now()
		}

		app.infoLog.Printf("Мердж-реквест слит, обновляем статус задачи на 'Готово' (время: %v)", updatedAt)
		err = app.models.UpdateSprintIssueStatus(
			sprintID,
			issueID,
			"Готово", // Явно указываем статус
			nil,
			&updatedAt,
			webhook.ObjectAttributes.SourceBranch,
			&webhook.ObjectAttributes.IID,
		)
		if err != nil {
			app.errorLog.Printf("Ошибка обновления статуса задачи: %v", err)
			return err
		}
		app.infoLog.Printf("Статус задачи %d успешно обновлен после мерджа", issueID)

	case "opened", "reopened":
		app.infoLog.Printf("Мердж-реквест открыт/переоткрыт, обновляем статус задачи на 'На проверке'")
		err = app.models.UpdateSprintIssueStatus(
			sprintID,
			issueID,
			"На проверке", // Явно указываем статус
			nil,
			nil,
			webhook.ObjectAttributes.SourceBranch,
			&webhook.ObjectAttributes.IID,
		)
		if err != nil {
			app.errorLog.Printf("Ошибка обновления статуса задачи: %v", err)
			return err
		}
		app.infoLog.Printf("Статус задачи %d обновлен на 'На проверке'", issueID)

	case "closed":
		app.infoLog.Printf("Мердж-реквест закрыт без слияния")
		// Можно добавить логику для обработки закрытого без слияния MR

	default:
		app.infoLog.Printf("Неизвестное состояние мердж-реквеста: %s", webhook.ObjectAttributes.State)
	}

	return nil
}

func (app *application) getSprintIssue(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		app.errorLog.Printf("Неверный формат ID спринта: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
		return
	}

	issueID, err := strconv.Atoi(c.Param("taskId"))
	if err != nil {
		app.errorLog.Printf("Неверный формат ID задачи: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID задачи"})
		return
	}

	// Получаем информацию о задаче из базы данных
	issue, err := app.models.GetSprintIssue(sprintID, issueID)
	if err != nil {
		app.errorLog.Printf("Ошибка при получении задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить информацию о задаче"})
		return
	}

	if issue == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена"})
		return
	}

	// Получаем дополнительную информацию из GitLab
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	// Получаем информацию о коммитах и мердж-реквестах из GitLab
	gitlabURL := fmt.Sprintf("%s/api/v4/projects/%s/issues/%d", app.oauthHandler.gitlabBaseURL, c.Param("id"), issueID)
	req, err := http.NewRequest("GET", gitlabURL, nil)
	if err != nil {
		app.errorLog.Printf("Ошибка создания запроса к GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}

	req.Header.Set("Authorization", token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		app.errorLog.Printf("Ошибка запроса к GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	var gitlabIssue map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&gitlabIssue); err != nil {
		app.errorLog.Printf("Ошибка парсинга ответа от GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обработки ответа от GitLab"})
		return
	}

	// Объединяем данные из базы и GitLab
	response := map[string]interface{}{
		"id": issue.IssueID,
		"title": issue.Title,
		"description": issue.Description,
		"status": issue.Status,
		"priority": issue.Priority,
		"assigned_to": issue.AssignedTo,
		"si_last_commit": issue.LastCommit,
		"si_last_merge": issue.LastMerge,
		"si_branch_name": issue.BranchName,
		"si_mr_id": issue.MRID,
		"gitlab_data": gitlabIssue,
	}

	c.JSON(http.StatusOK, response)
}

// syncIssueStatusWithGitLab синхронизирует статус задачи с GitLab
func (app *application) syncIssueStatusWithGitLab(projectID int, issueID int, token string) error {
	// Получаем информацию о задаче из GitLab
	gitlabURL := fmt.Sprintf("%s/api/v4/projects/%d/issues/%d", app.oauthHandler.gitlabBaseURL, projectID, issueID)
	req, err := http.NewRequest("GET", gitlabURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса к GitLab: %v", err)
	}

	req.Header.Set("Authorization", token)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка запроса к GitLab: %v", err)
	}
	defer resp.Body.Close()

	var gitlabIssue struct {
		State string `json:"state"`
		IID   int    `json:"iid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gitlabIssue); err != nil {
		return fmt.Errorf("ошибка парсинга ответа от GitLab: %v", err)
	}

	// Если задача в GitLab закрыта, обновляем статус в нашей системе
	if gitlabIssue.State == "closed" {
		// Получаем спринт, в котором находится задача
		sprintID, err := app.models.GetSprintIDByIssueID(issueID)
		if err != nil {
			return fmt.Errorf("ошибка получения спринта для задачи %d: %v", issueID, err)
		}

		// Получаем текущий статус задачи
		issue, err := app.models.GetSprintIssue(sprintID, issueID)
		if err != nil {
			return fmt.Errorf("ошибка получения задачи %d: %v", issueID, err)
		}

		// Если статус не "Готово", обновляем его
		if issue.Status != "Готово" {
			app.infoLog.Printf("Синхронизация: задача #%d в GitLab закрыта, обновляем статус на 'Готово'", issueID)
			err = app.models.UpdateSprintIssueStatus(
				sprintID,
				issueID,
				"Готово",
				nil,
				nil,
				issue.BranchName,
				&gitlabIssue.IID,
			)
			if err != nil {
				return fmt.Errorf("ошибка обновления статуса задачи: %v", err)
			}
		}
	}

	return nil
}

// syncAllIssuesWithGitLab синхронизирует статусы всех задач в спринте с GitLab
func (app *application) syncAllIssuesWithGitLab(sprintID int, projectID int, token string) error {
	// Получаем все задачи спринта
	issues, err := app.models.GetSprintIssues(sprintID)
	if err != nil {
		return fmt.Errorf("ошибка получения задач спринта: %v", err)
	}

	// Синхронизируем каждую задачу
	for _, issue := range issues {
		if err := app.syncIssueStatusWithGitLab(projectID, issue.IssueID, token); err != nil {
			app.errorLog.Printf("Ошибка синхронизации задачи #%d: %v", issue.IssueID, err)
			// Продолжаем с другими задачами даже если одна не удалась
			continue
		}
	}

	return nil
}

// completeSprint обрабатывает запрос на завершение спринта
func (app *application) completeSprint(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
		return
	}

	// Проверяем существование спринта
	sprint, err := app.models.GetSprint(sprintID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Спринт не найден"})
		return
	}

	// Проверяем, не завершен ли уже спринт
	if sprint.SptStatus == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Спринт уже завершен"})
		return
	}

	// Завершаем спринт
	err = app.models.CompleteSprint(sprintID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка при завершении спринта: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"message": "Спринт успешно завершен",
	})
}

func (h *OAuthHandler) GitLabProjectMembersHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	projectID := c.Param("id")
	url := fmt.Sprintf("%s/api/v4/projects/%s/members", h.gitlabBaseURL, projectID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	var members []map[string]interface{}
	if err := json.Unmarshal(body, &members); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных"})
		return
	}

	// Получаем настройки пользователей из локальной БД
	var membersWithSettings []map[string]interface{}
	for _, member := range members {
		userID := int(member["id"].(float64))
		
		// Получаем настройки пользователя из локальной БД
		var settings struct {
			Role string `json:"role"`
		}
		
		// Получаем настройки пользователя из БД
		err := h.app.db.QueryRow(context.Background(), `
			SELECT us_role 
			FROM user_settings 
			WHERE us_user_id = $1
		`, userID).Scan(&settings.Role)
		
		// Если настройки не найдены, используем значение по умолчанию
		if err != nil {
			if err == sql.ErrNoRows {
				settings.Role = "developer"
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения настроек пользователя"})
				return
			}
		}
		
		// Объединяем данные из GitLab и локальной БД
		memberWithSettings := map[string]interface{}{
			"id": member["id"],
			"name": member["name"],
			"username": member["username"],
			"email": member["email"],
			"avatar_url": member["avatar_url"],
			"created_at": member["created_at"],
			"userSettings": map[string]interface{}{
				"us_role": settings.Role,
			},
		}
		
		membersWithSettings = append(membersWithSettings, memberWithSettings)
	}

	c.JSON(http.StatusOK, membersWithSettings)
}

type UpdateUserRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

func (h *OAuthHandler) UpdateUserRoleHandler(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID пользователя не указан"})
		return
	}

	var req UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	// Проверяем валидность роли
	validRoles := map[string]bool{
		"administrator":   true,
		"project_manager": true,
		"developer":      true,
	}

	if !validRoles[req.Role] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Неверная роль пользователя. Допустимые роли: %v", getValidRoles()),
		})
		return
	}

	// Обновляем роль пользователя в БД
	query := `
		INSERT INTO user_settings (us_user_id, us_role)
		VALUES ($1, $2)
		ON CONFLICT (us_user_id) 
		DO UPDATE SET 
			us_role = $2,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := h.app.db.Exec(context.Background(), query, userID, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления роли пользователя"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Роль пользователя успешно обновлена"})
}

func getValidRoles() []string {
	return []string{"project_manager", "developer"}
}

type CreateGitLabProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Visibility  string `json:"visibility" binding:"required"`
	StartDate   string `json:"start_date" binding:"required"`
	EndDate     string `json:"end_date" binding:"required"`
}

// transliterate преобразует кириллицу в латиницу
func transliterate(s string) string {
	translitMap := map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
		'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
		'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
		'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "",
		'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
		'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D", 'Е': "E", 'Ё': "Yo",
		'Ж': "Zh", 'З': "Z", 'И': "I", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M",
		'Н': "N", 'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U",
		'Ф': "F", 'Х': "H", 'Ц': "Ts", 'Ч': "Ch", 'Ш': "Sh", 'Щ': "Sch", 'Ъ': "",
		'Ы': "Y", 'Ь': "", 'Э': "E", 'Ю': "Yu", 'Я': "Ya",
	}

	var result strings.Builder
	for _, r := range s {
		if val, ok := translitMap[r]; ok {
			result.WriteString(val)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// CreateGitLabProject создает новый проект в GitLab
func (h *OAuthHandler) CreateGitLabProject(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	var req CreateGitLabProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.app.errorLog.Printf("Ошибка валидации данных: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Неверный формат данных: %v", err)})
		return
	}

	// Логируем полученные данные
	h.app.infoLog.Printf("Получен запрос на создание проекта: %+v", req)

	// Формируем URL для создания проекта в GitLab
	url := fmt.Sprintf("%s/api/v4/projects", h.gitlabBaseURL)

	// Генерируем безопасный путь для проекта
	safePath := strings.ToLower(transliterate(req.Name))
	// Заменяем пробелы и специальные символы на дефисы
	safePath = strings.ReplaceAll(safePath, " ", "-")
	safePath = strings.ReplaceAll(safePath, "_", "-")
	// Удаляем все символы кроме букв, цифр и дефисов
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	safePath = reg.ReplaceAllString(safePath, "")
	// Удаляем множественные дефисы
	safePath = strings.ReplaceAll(safePath, "--", "-")
	// Удаляем дефисы в начале и конце
	safePath = strings.Trim(safePath, "-")
	// Добавляем временную метку для уникальности
	safePath = fmt.Sprintf("%s-%d", safePath, time.Now().Unix())

	// Создаем тело запроса
	projectData := map[string]interface{}{
		"name":                 req.Name,
		"description":          req.Description,
		"visibility":           req.Visibility,
		"initialize_with_readme": true,
		"default_branch":       "main",
		"path":                safePath,
	}

	jsonData, err := json.Marshal(projectData)
	if err != nil {
		h.app.errorLog.Printf("Ошибка маршалинга данных: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	// Логируем данные, отправляемые в GitLab
	h.app.infoLog.Printf("Отправляем данные в GitLab: %s", string(jsonData))

	// Создаем HTTP запрос
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		h.app.errorLog.Printf("Ошибка создания HTTP запроса: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	request.Header.Set("Authorization", token)
	request.Header.Set("Content-Type", "application/json")

	// Отправляем запрос
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		h.app.errorLog.Printf("Ошибка отправки запроса в GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		h.app.errorLog.Printf("Ошибка чтения ответа от GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	// Логируем ответ от GitLab
	h.app.infoLog.Printf("Ответ от GitLab (статус %d): %s", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusCreated {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			h.app.errorLog.Printf("Ошибка от GitLab: %v", errorResponse)
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			h.app.errorLog.Printf("Неизвестная ошибка от GitLab: %s", string(body))
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	// Парсим ответ от GitLab
	var gitlabProject map[string]interface{}
	if err := json.Unmarshal(body, &gitlabProject); err != nil {
		h.app.errorLog.Printf("Ошибка парсинга ответа от GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга ответа"})
		return
	}

	// Возвращаем успешный ответ
	c.JSON(http.StatusCreated, gin.H{
		"message": "Проект успешно создан",
		"project": gin.H{
			"gitlab_id":   gitlabProject["id"],
			"name":        req.Name,
			"description": req.Description,
			"start_date":  req.StartDate,
			"end_date":    req.EndDate,
			"visibility":  req.Visibility,
			"web_url":     gitlabProject["web_url"],
		},
	})
}

type UpdateProjectRequest struct {
	Description string `json:"description"`
}

func (h *OAuthHandler) UpdateGitLabProject(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	projectID := c.Param("id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID проекта не указан"})
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.app.errorLog.Printf("Ошибка валидации данных: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Неверный формат данных: %v", err)})
		return
	}

	// Формируем URL для обновления проекта в GitLab
	url := fmt.Sprintf("%s/api/v4/projects/%s", h.gitlabBaseURL, projectID)

	// Создаем тело запроса
	projectData := map[string]interface{}{
		"description": req.Description,
	}

	jsonData, err := json.Marshal(projectData)
	if err != nil {
		h.app.errorLog.Printf("Ошибка маршалинга данных: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	// Создаем HTTP запрос
	request, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		h.app.errorLog.Printf("Ошибка создания HTTP запроса: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	request.Header.Set("Authorization", token)
	request.Header.Set("Content-Type", "application/json")

	// Отправляем запрос
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		h.app.errorLog.Printf("Ошибка отправки запроса в GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		h.app.errorLog.Printf("Ошибка чтения ответа от GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			h.app.errorLog.Printf("Ошибка от GitLab: %v", errorResponse)
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			h.app.errorLog.Printf("Неизвестная ошибка от GitLab: %s", string(body))
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	// Парсим ответ от GitLab
	var gitlabProject map[string]interface{}
	if err := json.Unmarshal(body, &gitlabProject); err != nil {
		h.app.errorLog.Printf("Ошибка парсинга ответа от GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга ответа"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Описание проекта успешно обновлено",
		"project": gin.H{
			"gitlab_id":   gitlabProject["id"],
			"name":        gitlabProject["name"],
			"description": gitlabProject["description"],
			"web_url":     gitlabProject["web_url"],
		},
	})
}

func (h *OAuthHandler) DeleteGitLabProject(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	projectID := c.Param("id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID проекта не указан"})
		return
	}

	// Формируем URL для удаления проекта в GitLab
	url := fmt.Sprintf("%s/api/v4/projects/%s", h.gitlabBaseURL, projectID)

	// Создаем HTTP запрос
	request, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		h.app.errorLog.Printf("Ошибка создания HTTP запроса: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}

	request.Header.Set("Authorization", token)

	// Отправляем запрос
	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		h.app.errorLog.Printf("Ошибка отправки запроса в GitLab: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			h.app.errorLog.Printf("Ошибка от GitLab: %v", errorResponse)
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			h.app.errorLog.Printf("Неизвестная ошибка от GitLab: %s", string(body))
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Проект успешно удален",
	})
}

// DeleteGitLabIssue удаляет задачу через GitLab API
func (h *OAuthHandler) DeleteGitLabIssue(c *gin.Context) {
	projectID := c.Param("id")
	issueID := c.Param("issueId")
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Отсутствует токен авторизации"})
		return
	}

	// Удаляем префикс "Bearer " из токена, если он есть
	token = strings.TrimPrefix(token, "Bearer ")

	url := fmt.Sprintf("%s/api/v4/projects/%s/issues/%s", h.gitlabBaseURL, projectID, issueID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при создании запроса"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при выполнении запроса"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{"error": fmt.Sprintf("Ошибка при удалении задачи: %s", string(body))})
		return
	}

	c.Status(http.StatusNoContent)
}

// GetUsersHandler возвращает список всех пользователей
func (h *OAuthHandler) GetUsersHandler(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Токен отсутствует"})
		return
	}

	// Получаем список пользователей из GitLab
	url := fmt.Sprintf("%s/api/v4/users", h.gitlabBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания запроса"})
		return
	}
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных"})
		return
	}

	// Получаем настройки пользователей из локальной БД
	var usersWithSettings []map[string]interface{}
	for _, user := range users {
		userID := int(user["id"].(float64))
		
		// Получаем настройки пользователя из локальной БД
		var settings struct {
			Role string `json:"role"`
		}
		
		// Получаем настройки пользователя из БД
		err := h.app.db.QueryRow(context.Background(), `
			SELECT us_role 
			FROM user_settings 
			WHERE us_user_id = $1
		`, userID).Scan(&settings.Role)
		
		// Если настройки не найдены, используем значение по умолчанию
		if err != nil {
			if err == sql.ErrNoRows {
				settings.Role = "developer"
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения настроек пользователя"})
				return
			}
		}
		
		// Объединяем данные из GitLab и локальной БД
		userWithSettings := map[string]interface{}{
			"id": user["id"],
			"name": user["name"],
			"username": user["username"],
			"email": user["email"],
			"avatar_url": user["avatar_url"],
			"created_at": user["created_at"],
			"userSettings": map[string]interface{}{
				"us_role": settings.Role,
			},
		}
		
		usersWithSettings = append(usersWithSettings, userWithSettings)
	}

	c.JSON(http.StatusOK, usersWithSettings)
}

func (app *application) updateIssueStatus(c *gin.Context) {
	sprintID, err := strconv.Atoi(c.Param("sprintId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID спринта"})
		return
	}

	issueID, err := strconv.Atoi(c.Param("issueId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат ID задачи"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	// Проверяем валидность статуса
	validStatuses := []string{"К выполнению", "В работе", "На проверке", "Готово", "Заблокировано"}
	isValid := false
	for _, status := range validStatuses {
		if status == req.Status {
			isValid = true
			break
		}
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный статус задачи"})
		return
	}

	err = app.models.UpdateIssueStatus(sprintID, issueID, req.Status)
	if err != nil {
		app.errorLog.Printf("Ошибка обновления статуса задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить статус задачи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (app *application) deleteSprintIssue(c *gin.Context) {
	sprintID := c.Param("sprintID")
	issueID := c.Param("issueID")

	sprintIDInt, err := strconv.Atoi(sprintID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sprint ID"})
		return
	}

	issueIDInt, err := strconv.Atoi(issueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid issue ID"})
		return
	}

	// Delete the issue from the sprint
	err = app.models.DeleteSprintIssue(sprintIDInt, issueIDInt)
	if err != nil {
		app.errorLog.Printf("Error deleting sprint issue: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete sprint issue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sprint issue deleted successfully"})
}
