package models

import (
	"errors"
)

var ErrNoRecord = errors.New("models: подходящей записи не найдено!")

type User struct {
    UsrID           int `db:"usr_id"`
    UsrUsername  string `db:"usr_username"`   // Поле для хранения логина пользователя
    UsrEmail     string `db:"usr_email"`      // Поле для хранения email
    UsrPassword  string `db:"usr_password"`   // Поле для хранения пароля
    UsrRole      string `db:"usr_role"`       // Роль пользователя
    UsrName      string `db:"usr_name"`       // Имя пользователя
    UsrPatronomic string `db:"usr_patronomic"`// Отчество пользователя
    UsrSurname   string `db:"usr_surname"`    // Фамилия пользователя
}