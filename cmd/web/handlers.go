package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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
		fmt.Println("Ошибка запроса к GitLab:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка запроса к GitLab"})
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body) // Исправлено: читаем resp.Body, а не req.Body
	if err != nil {
		fmt.Println("Ошибка чтения ответа:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка чтения ответа"})
		return
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			c.JSON(resp.StatusCode, gin.H{"error": errorResponse["message"]})
		} else {
			c.JSON(resp.StatusCode, gin.H{"error": string(body)})
		}
		return
	}

	// Парсим данные как массив пользователей
	var users []map[string]interface{}
	if err := json.Unmarshal(body, &users); err != nil {
		fmt.Println("Ошибка парсинга данных пользователей:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка парсинга данных пользователей"})
		return
	}

	// Форматируем данные для фронтенда
	var members []map[string]interface{}
	for _, user := range users {
		members = append(members, map[string]interface{}{
			"id":         user["id"],
			"name":       user["name"],
			"email":      user["email"],
			"created_at": user["created_at"],
			"role":       "Developer", // GitLab не возвращает роль, можно получить через /api/v4/users/:id/impersonation_tokens
		})
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
}

func (app *application) addIssueToSprint(c *gin.Context) {
	var req AddIssueToSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный формат данных"})
		return
	}

	if req.SprintID == 0 || req.IssueID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Необходимо указать ID спринта и задачи"})
		return
	}

	err := app.models.AddIssueToSprint(req.SprintID, req.IssueID, req.StoryPoints, req.Priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
