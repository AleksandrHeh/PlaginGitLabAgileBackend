package models

import (
	"errors"
	"time"
)

var ErrNoRecord = errors.New("models: подходящей записи не найдено!")

type User struct {
	UsrID         int    `db:"usr_id"`
	UsrUsername   string `db:"usr_username"`   // Поле для хранения логина пользователя
	UsrEmail      string `db:"usr_email"`      // Поле для хранения email
	UsrPassword   string `db:"usr_password"`   // Поле для хранения пароля
	UsrRole       string `db:"usr_role"`       // Роль пользователя
	UsrName       string `db:"usr_name"`       // Имя пользователя
	UsrPatronomic string `db:"usr_patronomic"` // Отчество пользователя
	UsrSurname    string `db:"usr_surname"`    // Фамилия пользователя
}

type Project struct {
	PrjID          int       `db:"prj_id"`
	PrjTitle       string    `db:"prj_title"`
	PrjDescription string    `db:"prj_description"`
	PrjStartDate   time.Time `db:"prj_start_date"`
	PrjEndDate     time.Time `db:"prj_end_date"`
	PrjStatus      string    `db:"prj_status"`
	PrjOwner       string    `db:"prj_owner"`
}

type Tasks struct {
	TskId          int     `db:"tsk_id"`
	TskPrjId       int     `db:"tsk_prj_id"`
	TskTitle       string  `db:"tsk_title"`
	TskDescription string  `db:"tsk_description"`
	TskPriority    string  `db:"tsk_priority"`
	TskStatus      string  `db:"tsk_status"`
	TskAssigneId   *string `db:"tsk_assignee_id"`
}

// SprintIssue представляет задачу в спринте
type SprintIssue struct {
    SprintID    int    `json:"sprint_id"`
    IssueID     int    `json:"issue_id"`
    StoryPoints int    `json:"story_points"`
    Priority    string `json:"priority"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Status      string `json:"status"`
    AssignedTo  *int   `json:"assigned_to"`
}
