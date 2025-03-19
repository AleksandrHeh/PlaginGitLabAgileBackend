package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
    "net/url"

    "github.com/gin-gonic/gin"
)

type GitLabUser struct {
    ID       int    `json:"id"`
    Username string `json:"username"`
    Name     string `json:"name"`
    Email    string `json:"email"`
}

type OAuthHandler struct {
    clientID     string
    clientSecret string
    redirectURI  string
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