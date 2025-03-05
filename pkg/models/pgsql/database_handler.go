package pgsql

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "golang.org/x/text/date"
	"golangify.com/plaginagile/pkg/models"
)

type PullIncludes struct {
	DB *pgxpool.Pool
}

func (pl *PullIncludes) IsValidUser(username, password string) (*models.User, error) {
	user := &models.User{}
	stmt := "SELECT usr_id, usr_username, usr_email, usr_password, usr_role, usr_name, usr_patronomic, usr_surname FROM users WHERE usr_username=$1"

	err := pl.DB.QueryRow(context.Background(), stmt, username).Scan(
		&user.UsrID,
		&user.UsrUsername,
		&user.UsrEmail,
		&user.UsrPassword,
		&user.UsrRole,
		&user.UsrName,
		&user.UsrPatronomic,
		&user.UsrSurname,
	)
	if err != nil {
		return nil, err
	}

	// Прямое сравнение пароля
	if user.UsrPassword != password {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

func (pl *PullIncludes) GetUsers() ([]models.User, error) {
	stmt := "SELECT usr_id ,usr_name, usr_patronomic, usr_surname, usr_role FROM users"
	rows, err := pl.DB.Query(context.Background(), stmt)
	if err != nil {
		log.Println("Ошибка выполнения запроса:", err)
		return nil, errors.New("Не удалось получить пользователей")
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.UsrID, &user.UsrName, &user.UsrPatronomic, &user.UsrSurname, &user.UsrRole); err != nil {
			log.Println("Ошибка чтения данных пользователя:", err)
			return nil, errors.New("ошибка чтения данных пользователя")
		}
		users = append(users, user)
	}
	return users, nil
}

func (pl *PullIncludes) CreateProject(name, description, startDate, endDate, prj_status string, ownerID int) (int, error) {
	var projectID int
	query := "INSERT INTO projects (prj_title, prj_description, prj_start_date, prj_end_date, prj_status, prj_owner) VALUES ($1, $2, $3, $4, $5, $6) RETURNING prj_id"
	err := pl.DB.QueryRow(context.Background(), query, name, description, startDate, endDate, prj_status, ownerID).Scan(&projectID)
	if err != nil {
		return projectID, fmt.Errorf("Failed to insert project: %w", err)
	}
	return projectID, nil
}

func (pl *PullIncludes) GetUser(user_id string) (string, error) {
	stmt := "SELECT CONCAT(usr_surname , ' ', usr_name, ' ', usr_patronomic) FROM users WHERE usr_id = $1"
    var SurnameNamePatronomic string
    err := pl.DB.QueryRow(context.Background(), stmt, user_id).Scan(&SurnameNamePatronomic)
    if err != nil {
        log.Println("Ошибка получения ФИО пользователя:", err)
        return "", errors.New("Не удалось получить ФИО пользователя")
    }
    return SurnameNamePatronomic, nil
}

func (pl *PullIncludes) UpdateProject(prj_title, prj_description, prj_start_date, prj_end_date string,  prj_id int) (error){
    query := "UPDATE projects SET prj_title = $1, prj_description = $2, prj_start_date = $3, prj_end_date = $4 WHERE prj_id = $5;"
    _, err := pl.DB.Exec(context.Background(), query, prj_title, prj_description, prj_start_date, prj_end_date, prj_id)
    if err != nil {
        return fmt.Errorf("не удалось изменить проект: %w", err)
    }
    return nil
}

func (pl *PullIncludes) GetProject(prj_id int) (models.Project, error) {
	var project models.Project
	query := "SELECT prj_title, prj_description, prj_start_date, prj_end_date, prj_status, prj_owner FROM projects WHERE prj_id = $1"
	err := pl.DB.QueryRow(context.Background(), query, prj_id).Scan(
		&project.PrjTitle, &project.PrjDescription, &project.PrjStartDate, &project.PrjEndDate, &project.PrjStatus, &project.PrjOwner)
	if err != nil {
		return models.Project{}, fmt.Errorf("не удалось получить данные проекта: %v", err)
	}
	return project, nil
}

// prj_id, prj_title, prj_description, prj_start_date, prj_end_date string, prj_status, prj_owner
func (pl *PullIncludes) GetProjects() ([]models.Project, error) {
	stmt := "SELECT prj_id, prj_title, prj_description, prj_start_date, prj_end_date, prj_status, prj_owner FROM projects"
	rows, err := pl.DB.Query(context.Background(), stmt)
	if err != nil {
		log.Println("Ошибка выполнения запроса:", err)
		return nil, errors.New("Не удалось выполнить запрос к базе данных")
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var project models.Project
		err := rows.Scan(
			&project.PrjID,
			&project.PrjTitle,
			&project.PrjDescription,
			&project.PrjStartDate,
			&project.PrjEndDate,
			&project.PrjStatus,
			&project.PrjOwner,
		)
		if err != nil {
			log.Println("Ошибка чтения строки:", err)
			return nil, errors.New("Ошибка обработки данных")
		}
		
		fmt.Printf(project.PrjTitle)

        ownerSurnameNamePatronomic, err := pl.GetUser(project.PrjOwner)
        if err != nil {
            return nil, errors.New("Ошибка получения пользователя")
        }
        
        project.PrjOwner = ownerSurnameNamePatronomic
        projects = append(projects, project)
	}

	if rows.Err() != nil {
		log.Println("Ошибка после обработки строк:", rows.Err())
		return nil, errors.New("Ошибка обработки данных после чтения")
	}

	return projects, nil
}

func (pl *PullIncludes) CreateTask(tsk_prj_id int, tsk_title, tsk_description, tsk_priority, tsk_status string) error{
	query := "INSERT INTO tasks (tsk_prj_id, tsk_title, tsk_description, tsk_priority, tsk_status) VALUES ($1, $2, $3, $4, $5)"
	_, err := pl.DB.Exec(context.Background(), query, tsk_prj_id, tsk_title, tsk_description, tsk_priority, tsk_status)
	if err != nil {
		return fmt.Errorf("Failef to add task to project: %w", err)
	}
	return nil
}

func (pl *PullIncludes) AddUsersProjects(projectID, userID int) error {
	query := "INSERT INTO users_projects (prt_prj_id, prt_usr_id) VALUES ($1, $2)"
	_, err := pl.DB.Exec(context.Background(), query, projectID, userID)
	if err != nil {
		return fmt.Errorf("Failef to add users to project: %w", err)
	}
	return nil
}

func (pl *PullIncludes) GetTasksProject(tsk_prj_id int) ([]models.Tasks, error){
	fmt.Print("dfdfd")
	query := "SELECT tsk_id, tsk_prj_id, tsk_title, tsk_description, tsk_priority, tsk_status, tsk_assignee_id FROM tasks WHERE tsk_prj_id = $1"
	rows, err := pl.DB.Query(context.Background(), query, tsk_prj_id)
	if err != nil {
		log.Println("Ошибка выполнения запроса:", err)
		return nil, errors.New("Не удалось выполнить запрос к базе данных")
	}
	defer rows.Close()
	fmt.Println("fffffff3")
	var tasks []models.Tasks
	for rows.Next(){
		var task models.Tasks
		err := rows.Scan(
			&task.TskId,
			&task.TskPrjId,
			&task.TskTitle,
			&task.TskDescription,
			&task.TskPriority,
			&task.TskStatus,
			&task.TskAssigneId,
		)
		if err != nil {
			log.Println("Ошибка чтения строки:", err)
			return nil, errors.New("Ошибка обработки данных")
		}
		tasks = append(tasks,task)
	} 
	

	if rows.Err() != nil {
		log.Println("Ошибка после обработки строк:", rows.Err())
		return nil, errors.New("Ошибка обработки данных после чтения")
	}
	return tasks, nil
}

func (pl *PullIncludes) DeleteProject(prj_id int) error {
	query := "DELETE FROM projects WHERE prj_id = $1"
	_, err := pl.DB.Exec(context.Background(), query, prj_id)
	if err != nil {
		return fmt.Errorf("Не удалось удалить проект: %w", err)
	}
	return err
}

func (pl *PullIncludes) DeleteTask(tsk_id int) error {
    query := "DELETE FROM tasks WHERE tsk_id = $1"
    _, err := pl.DB.Exec(context.Background(), query, tsk_id)
    if err != nil {
        return fmt.Errorf("не удалось удалить задачу: %w", err)
    }
    return nil
}