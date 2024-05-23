package database

import (
	"database/sql"
	"errors"
	"fmt"
	"go_final_project/models"
)

//var Db *sql.DB

/*func createTable(db *sql.DB) {
	_, err := db.Exec(
		"CREATE TABLE IF NOT EXISTS `scheduler` (`id` INTEGER PRIMARY KEY AUTOINCREMENT, `date` VARCHAR(8) NULL, `title` VARCHAR(64) NOT NULL, `comment` VARCHAR(255) NULL, `repeat` VARCHAR(128) NULL)")
	if err != nil {
		log.Fatal(err)
	}
}*/

func InsertTask(Db *sql.DB, task models.Task) (int, error) {
	result, err := Db.Exec("INSERT INTO scheduler (date, title, comment, repeat) VALUES (:date, :title, :comment, :repeat)",
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat))
	if err != nil {
		return 0, err
	}
	// стало
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
	//стало
	/*rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	if rowsAffected == 0 {
		return 0, errors.New("no rows inserted")
	}

	return int(rowsAffected), nil*/
}

// 5
func SearchTasks(Db *sql.DB, search string) ([]models.Task, error) {
	var tasks []models.Task

	search = fmt.Sprintf("%%%s%%", search)
	rows, err := Db.Query("SELECT * FROM scheduler WHERE title LIKE :search OR comment LIKE :search ORDER BY date",
		sql.Named("search", search))
	if err != nil {
		return []models.Task{}, err
	}
	defer rows.Close()
	//  добавляет повтор отображения задач в браузере
	for rows.Next() {
		var task models.Task
		if err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			return []models.Task{}, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return []models.Task{}, err
	}

	return tasks, nil
}
func SearchTasksByDate(Db *sql.DB, date string) ([]models.Task, error) {
	var tasks []models.Task

	rows, err := Db.Query("SELECT * FROM scheduler WHERE date = :date",
		sql.Named("date", date))
	if err != nil {
		return []models.Task{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var task models.Task
		if err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			return []models.Task{}, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return []models.Task{}, err
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	return tasks, nil
}
func ReadTasks(Db *sql.DB) ([]models.Task, error) {
	var tasks []models.Task

	rows, err := Db.Query("SELECT id, date, title, comment, repeat FROM scheduler ORDER BY date LIMIT 10")
	if err != nil {
		return []models.Task{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var task models.Task
		if err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
			return []models.Task{}, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return []models.Task{}, err
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	return tasks, nil
}

// 6 шаг
func UpdateTask(Db *sql.DB, task models.Task) (models.Task, error) {
	req, err := Db.Exec("UPDATE scheduler SET date = :date, title = :title, comment = :comment, repeat = :repeat WHERE id = :id",
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat),
		sql.Named("id", task.ID))
	if err != nil {
		return models.Task{}, err
	}

	rowsAffected, err := req.RowsAffected()
	if err != nil {
		return models.Task{}, err
	}

	if rowsAffected == 0 {
		return models.Task{}, errors.New("не удалось обновить")
	}

	return task, nil
}
func ReadTask(Db *sql.DB, id string) (models.Task, error) {
	var task models.Task

	row := Db.QueryRow("SELECT * FROM scheduler WHERE id = :id",
		sql.Named("id", id))
	if err := row.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat); err != nil {
		return models.Task{}, err
	}

	return task, nil
}

// 7 шаг
func DeleteTaskDb(Db *sql.DB, id string) error {
	result, err := Db.Exec("DELETE FROM scheduler WHERE id = :id",
		sql.Named("id", id))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("не удалось удалить")
	}

	return err
}
