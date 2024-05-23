package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"go_final_project/database"
	"go_final_project/models"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"
	_ "github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

const returnDateInFormat string = "20060102"

var Db *sql.DB

func NextDate(now time.Time, date string, repeat string) (string, error) {
	if len(repeat) == 0 {
		return "", errors.New("переменная repaet, пустая строка")
	}

	day, _ := regexp.MatchString(`d \d{1,3}`, repeat)
	year, _ := regexp.MatchString(`y`, repeat)
	week, _ := regexp.MatchString(`w [1-7]+(,[1-7])*`, repeat)

	if day {
		days, err := strconv.Atoi(strings.TrimPrefix(repeat, "d "))
		if err != nil {
			return "", err
		}

		if days > 400 {
			return "", errors.New("максимальное количество дней должно быть не больше 400")
		}

		parsedDate, err := time.Parse(returnDateInFormat, date)
		if err != nil {
			return "", err
		}

		newDate := parsedDate.AddDate(0, 0, days)

		for newDate.Before(now) {
			newDate = newDate.AddDate(0, 0, days)
		}

		return newDate.Format(returnDateInFormat), nil
	} else if year {
		parsedDate, err := time.Parse(returnDateInFormat, date)
		if err != nil {
			return "", err
		}

		newDate := parsedDate.AddDate(1, 0, 0)

		for newDate.Before(now) {
			newDate = newDate.AddDate(1, 0, 0)
		}

		return newDate.Format(returnDateInFormat), nil
	} else if week {
		parsedDate, err := time.Parse(returnDateInFormat, date)
		weekday := int(parsedDate.Weekday())
		if err != nil {
			return "", err
		}

		var newDate time.Time
		var weekdays []int

		for _, weekdayString := range strings.Split(strings.TrimPrefix(repeat, "w "), ",") {
			weekdayInt, _ := strconv.Atoi(weekdayString)
			weekdays = append(weekdays, weekdayInt)
		}

		updated := false
		for _, v := range weekdays {
			if weekday < v {
				newDate = parsedDate.AddDate(0, 0, v-weekday)
				updated = true
				break
			}
		}

		if !updated {
			newDate = parsedDate.AddDate(0, 0, 7-weekday+weekdays[0])
		}

		for newDate.Before(now) || newDate == now {
			weekday = int(newDate.Weekday())

			if weekday == weekdays[0] {
				for _, v := range weekdays {
					if weekday < v {
						newDate = newDate.AddDate(0, 0, v-weekday)
						weekday = int(newDate.Weekday())
					}
				}
			} else {
				newDate = newDate.AddDate(0, 0, 7-weekday+weekdays[0])
			}
		}

		return newDate.Format(returnDateInFormat), nil
	}
	return "", errors.New(" неправильный формат повтора")
}

// NextDateHandler обрабатывает запрос на получение следующей даты.
// Ожидает три параметра:
//   - now: текущая дата в формате "20060102"
//   - date: дата, на которую нужно перенести задачу
//   - repeat: формат повтора задачи (например, "d 10", "y", "w 1,3")
//
// Если хотя бы один из параметров отсутствует, возвращает ошибку 400.
func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка наличия параметров
	now := r.FormValue("now")
	date := r.FormValue("date")
	repeat := r.FormValue("repeat")

	if now == "" || date == "" || repeat == "" {
		http.Error(w, "Отсутствуют параметры", http.StatusBadRequest)
		return
	}

	// Парсинг текущей даты
	// Если не удаётся распарсить, возвращаем ошибку 400
	parsedNow, err := time.Parse("20060102", now)
	if err != nil {
		http.Error(w, "Неверный параметр «now»", http.StatusBadRequest)
		return
	}

	// Вызов функции NextDate с передачей параметров
	// Если возвращается ошибка, возвращаем ошибку 400
	nextDate, err := NextDate(parsedNow, date, repeat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Возвращаем ответ с следующей датой
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(nextDate))
}

func responseWithError(w http.ResponseWriter, err error) {
	//Обработка ошибок
	if err != nil {
		log.Printf("error: %v", err)
	}

	//Ответ с описанием ошибки
	error, _ := json.Marshal(models.ResponseError{Error: err.Error()})

	//Заголовок ответа
	w.Header().Set("Content-Type", "application/json")

	//Запись данных в ответ
	_, err = w.Write(error)

	//Если есть ошибка при записи в ответ,
	//то сообщить об этом с кодом ошибки BadRequest
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

// TaskPost обрабатывает POST-запрос на добавление задачи.
// Принимает JSON-объект с полями Title, Date и Repeat.
// Если в запросе отсутствует поле Title, то задает ему значение пустой строки.
// Если в запросе отсутствует поле Date, то задает ему текущую дату.
// Если в запросе присутствует поле Repeat, то вызывает функцию NextDate с текущей датой,
// полученной из поля Date, и повторением из поля Repeat. Если результат больше текущей даты,
// то задает ему значение результата NextDate.
// В итоге добавляет задачу в базу данных и возвращает идентификатор новой задачи.
func TaskPost(w http.ResponseWriter, r *http.Request) {
	// Создание переменной для хранения задачи
	var task models.Task
	// Создание буфера для чтения тела запроса
	var buf bytes.Buffer

	// Чтение тела запроса в буфер
	if _, err := buf.ReadFrom(r.Body); err != nil {
		// Если произошла ошибка чтения, отправляем ответ с описанием ошибки
		responseWithError(w, err)
		return
	}

	// Декодирование JSON-объекта в структуру модели
	if err := json.Unmarshal(buf.Bytes(), &task); err != nil {
		// Если произошла ошибка декодирования, отправляем ответ с описанием ошибки
		responseWithError(w, err)
		return
	}

	// Проверка корректности заполнения полей
	if len(task.Title) == 0 {
		// Если поле Title пустое, отправляем ответ с описанием ошибки
		responseWithError(w, errors.New("заголовок пуст"))
		return
	}

	if len(task.Date) == 0 {
		// Если поле Date пустое, задаем ему текущую дату
		task.Date = time.Now().Format(returnDateInFormat)
	} else {
		// Если поле Date не пустое, проверяем его корректность и вызываем функцию NextDate
		_, err := time.Parse(returnDateInFormat, task.Date)
		if err != nil {
			// Если дата некорректна, отправляем ответ с описанием ошибки
			responseWithError(w, errors.New("неправильная дата"))
			return
		}

		if len(task.Repeat) > 0 {
			// Если поле Repeat не пустое, вызываем функцию NextDate
			if nextDate, err := NextDate(time.Now(), task.Date, task.Repeat); err != nil {
				// Если произошла ошибка, отправляем ответ с описанием ошибки
				responseWithError(w, err)
				return
			} else if task.Date < time.Now().Format(returnDateInFormat) {
				// Если результат NextDate меньше текущей даты, задаем ему значение результата NextDate
				task.Date = nextDate
			}
		}

		if task.Date < time.Now().Format(returnDateInFormat) {
			// Если дата меньше текущей даты, задаем ей значение текущей даты
			task.Date = time.Now().Format(returnDateInFormat)
		}
	}

	// Добавление задачи в базу данных и получение идентификатора новой задачи
	taskId, err := database.InsertTask(Db, task)
	if err != nil {
		// Если произошла ошибка, отправляем ответ с описанием ошибки
		responseWithError(w, err)
		return
	}

	// Создание JSON-объекта с идентификатором новой задачи и отправка его в ответ
	taskIdData, err := json.Marshal(models.ResponseTaskId{Id: uint(taskId)})
	if err != nil {
		responseWithError(w, err)
		return
	}

	// Запись заголовка ответа и данных в ответ
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusCreated)
	_, err = w.Write(taskIdData)

	// Вывод идентификатора новой задачи в лог
	log.Printf("Добавлено задание с id=%d", taskId)

	if err != nil {
		// Если произошла ошибка при записи, отправляем ответ с описанием ошибки
		responseWithError(w, errors.New("ошибка идентификатора задачи записи"))
	}
}

// TasksRead обрабатывает запрос на чтение задач.
// Если в запросе есть параметр «search», то выполняется поиск задач,
// содержащих в заголовке или комментарии строку, совпадающую с «search».
// Если в запросе есть параметр «date», то выполняется поиск задач,
// созданных в указанную дату.
// В противном случае возвращаются все задачи.
// В ответ клиенту отправляется JSON-объект с массивом задач.
func TasksRead(w http.ResponseWriter, r *http.Request) {
	var tasks []models.Task
	var err error
	var date time.Time

	// Поисковое значение
	search := r.URL.Query().Get("search")

	// Получение задач из БД
	if search == "" {
		tasks, err = database.ReadTasks(Db)
		if err != nil {
			responseWithError(w, errors.New("не удалось получить задания"))
			return
		}
	} else {

		date, err = time.Parse("02.01.2006", search)
		if err != nil {
			// Поиск по заголовку и комментарию
			tasks, err = database.SearchTasks(Db, search)
			if err != nil {
				responseWithError(w, errors.New("не удалось получить задания"))
				return
			}
		} else {
			// Поиск по дате
			tasks, err = database.SearchTasksByDate(Db, date.Format(models.DatePattern))
			if err != nil {
				responseWithError(w, errors.New("не удалось получить задания"))
				return
			}
		}
	}
	// Конвертация в JSON и отправка клиенту
	jsonData, err := json.Marshal(map[string][]models.Task{"tasks": tasks})
	if err != nil {
		http.Error(w, "Не удалось закодировать задачи в JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

// TaskUpdate обрабатывает запрос на обновление задачи.
// Запрос должен содержать JSON-объект с полями ID, Date, Title и Repeat.
// Если запрос не валиден, возвращается ошибка.
// В противном случае, обновляется задача в базе данных и отправляется JSON-объект с обновленной задачей.
func TaskUpdate(w http.ResponseWriter, r *http.Request) {
	// Создание буфера для чтения тела запроса
	var task models.Task
	var buffer bytes.Buffer

	// Чтение тела запроса в буфер
	if _, err := buffer.ReadFrom(r.Body); err != nil {
		responseWithError(w, errors.New("ошибка чтения тела запроса"))
		return
	}

	// Декодирование JSON-объекта в структуру модели
	if err := json.Unmarshal(buffer.Bytes(), &task); err != nil {
		responseWithError(w, errors.New("ошибка декодирования JSON"))
		return
	}

	// Проверка валидности идентификатора задачи
	if task.ID == 0 {
		responseWithError(w, errors.New("пустой идентификатор задачи"))
		return
	}
	if _, err := strconv.Atoi(strconv.FormatUint(uint64(task.ID), 10)); err != nil {
		responseWithError(w, errors.New("неверный идентификатор задачи"))
		return
	}

	// Проверка валидности даты
	if _, err := time.Parse(models.DatePattern, task.Date); err != nil {
		responseWithError(w, errors.New("недействительная дата"))
		return
	}

	// Проверка валидности заголовка и повтора
	if len(task.Title) == 0 {
		responseWithError(w, errors.New("пустой заголовок задачи"))
		return
	}
	if len(task.Repeat) > 0 {
		if _, err := NextDate(time.Now(), task.Date, task.Repeat); err != nil {
			responseWithError(w, errors.New("неверный формат повтора"))
			return
		}
	}

	// Обновление задачи в базе данных
	updatedTask, err := database.UpdateTask(Db, task)
	if err != nil {
		responseWithError(w, errors.New("не удалось обновить задачу: недействительный заголовок"))
		return
	}

	// Кодирование обновленной задачи в JSON
	taskIdData, err := json.Marshal(updatedTask)
	if err != nil {
		responseWithError(w, errors.New("ошибка кодирования задачи в JSON"))
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(taskIdData)

	log.Printf("обновление задачи с id=%d", updatedTask.ID)
	if err != nil {
		responseWithError(w, errors.New("ошибка обновления задачи"))
		return
	}
}

// TaskReadGET обрабатывает запрос на чтение задачи по ее идентификатору.
// Запрос должен содержать параметр «id», определяющий идентификатор задачи.
// Если задача не найдена, возвращается ошибка.
// В противном случае, отправляется JSON-объект с задачей.
func TaskReadGET(w http.ResponseWriter, r *http.Request) {
	// Получение идентификатора задачи из запроса
	id := r.URL.Query().Get("id")

	// Чтение задачи из базы данных по идентификатору
	task, err := database.ReadTask(Db, id)
	if err != nil {
		responseWithError(w, errors.New("не удалось получить задание"))
		return
	}

	// Кодирование задачи в JSON
	tasksData, err := json.Marshal(task)
	if err != nil {
		responseWithError(w, errors.New("ошибка кодирования задания"))
	}

	// Установка заголовка Content-Type и статуса ответа
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	// Запись данных в ответ
	_, err = w.Write(tasksData)

	// Если возникла ошибка при записи данных,
	// то сообщить об этом с кодом ошибки BadRequest
	log.Printf("прочитано задание с id=%s", id)
	if err != nil {
		responseWithError(w, errors.New("ошибка записи задания"))
	}

	// Логирование успешного чтения задачи
	log.Printf("прочитано задание с id=%s", id)
}

// TaskDonePOST обрабатывает запрос на завершение задачи.
// Запрос должен содержать параметр «id», определяющий идентификатор задачи.
// Если задача повторяется, то дата задачи обновляется на следующую по расписанию.
// Если задача не повторяется, то она удаляется из базы данных.
// В противном случае, отправляется пустой JSON-объект.
func TaskDonePOST(w http.ResponseWriter, r *http.Request) {
	// Чтение задачи из базы данных по идентификатору
	task, err := database.ReadTask(Db, r.URL.Query().Get("id"))
	if err != nil {
		responseWithError(w, errors.New("не удалось получить задание"))
		return
	}

	// Проверка повторяемости задачи
	if len(task.Repeat) == 0 {
		// Удаление задачи из базы данных
		err = database.DeleteTaskDb(Db, strconv.FormatUint(uint64(task.ID), 10))
		if err != nil {
			responseWithError(w, errors.New("не удалось удалить задачу"))
			return
		}
	} else {
		// Обновление даты задачи на следующую по расписанию
		task.Date, err = NextDate(time.Now(), task.Date, task.Repeat)
		if err != nil {
			responseWithError(w, errors.New("не удалось получить следующую дату"))
			return
		}

		// Обновление задачи в базе данных
		_, err = database.UpdateTask(Db, task)
		if err != nil {
			responseWithError(w, errors.New("не удалось обновить задачу"))
			return
		}
	}

	// Кодирование пустого JSON-объекта
	tasksData, err := json.Marshal(struct{}{})
	if err != nil {
		responseWithError(w, errors.New("ошибка кодирования задания"))
	}

	// Установка заголовка Content-Type и статуса ответа
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	// Запись данных в ответ
	_, err = w.Write(tasksData)

	// Если возникла ошибка при записи данных,
	// то сообщить об этом с кодом ошибки BadRequest
	log.Printf("Выполнено задание с id=%d", task.ID)
	if err != nil {
		responseWithError(w, errors.New("ошибка записи задания"))
	}
}

// TaskDELETE обрабатывает запрос на удаление задачи.
// Запрос должен содержать параметр «id», определяющий идентификатор задачи.
// Если задача удалена из базы данных, то отправляется пустой JSON-объект.
// В противном случае, отправляется ошибка.
func TaskDELETE(w http.ResponseWriter, r *http.Request) {
	// Получение идентификатора задачи из запроса
	id := r.URL.Query().Get("id")

	// Удаление задачи из базы данных
	err := database.DeleteTaskDb(Db, id)
	if err != nil {
		responseWithError(w, errors.New("не удалось удалить задачу"))
		return
	}

	// Кодирование пустого JSON-объекта
	tasksData, err := json.Marshal(struct{}{})
	if err != nil {
		responseWithError(w, errors.New("ошибка кодирования задания"))
	}

	// Установка заголовка Content-Type и статуса ответа
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	// Запись данных в ответ
	_, err = w.Write(tasksData)

	// Если возникла ошибка при записи данных,
	// то сообщить об этом с кодом ошибки BadRequest
	log.Printf("Удалена задача с id=%s", id)
	if err != nil {
		responseWithError(w, errors.New("ошибка записи задания"))
		return
	}
}

// В main - инициализируем базу данных и настраивает HTTP-сервер.
func main() {
	// Задаем имя файла базы данных из переменной среды.
	os.Setenv("TODO_DBFILE", "scheduler.db")
	dbFile := os.Getenv("TODO_DBFILE")
	if dbFile == "" {
		log.Fatal("TODO_DBFILE environment variable is not set")
	}

	// Открываем соединение с базой данных.
	var err error
	Db, err = sql.Open("sqlite", dbFile)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Закрываем соединение с базой данных при выходе из программы.
	defer Db.Close()

	// Создайте таблицу «scheduler», если она не существует.
	_, err = Db.Exec(`CREATE TABLE IF NOT EXISTS scheduler (
	        id INTEGER PRIMARY KEY AUTOINCREMENT,
	        date TEXT NOT NULL,
	        title TEXT NOT NULL,
	        comment TEXT,
	        repeat TEXT(128)
	        )`)
	if err != nil {
		log.Println("Failed to create the database:", err)
		return
	}

	// Создаем индекс в столбце «дата».
	_, err = Db.Exec(`CREATE INDEX IF NOT EXISTS indexdate ON scheduler (date)`)
	if err != nil {
		log.Println("Failed to create the index:", err)
	}
	fmt.Println("Запускаем сервер")
	// Настройка HTTP-сервер.
	mux := http.NewServeMux()
	r := chi.NewRouter()
	const webDir = "./web"
	r.Mount("/", http.FileServer(http.Dir(webDir)))
	r.Mount("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))

	// Определяем маршруты API.

	//Получаем следующую дату для задачи на основе шаблона повторения.
	r.Get("/api/nextdate", NextDateHandler)

	// Добавляем новую задачу.
	r.Post("/api/task", TaskPost)

	// Получаем список задач
	r.Get("/api/tasks", TasksRead)

	// Получаем конкретную задачу по ID.
	r.Get("/api/task", TaskReadGET)

	// Обновляем существующую задачу.
	r.Put("/api/task", TaskUpdate)

	// Отмемаем задачу как выполненную.
	r.Post("/api/task/done", TaskDonePOST)

	// Удаляем задачу.
	r.Delete("/api/task", TaskDELETE)

	// Настройка обработчика по умолчанию для других маршрутов.
	mux.Handle("/api/", r)

	// Настройка обработчик по умолчанию для других маршрутов.
	combinedHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "./web/index.html")
		} else if strings.HasPrefix(r.URL.Path, "/web/") {
			http.StripPrefix("/web", http.FileServer(http.Dir("./web"))).ServeHTTP(w, r)
		} else {
			http.ServeFile(w, r, "./web/login.html")
		}
	}
	mux.HandleFunc("/", combinedHandler)

	// Установка номера порта из переменной среды
	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = "7540"
	}

	// Проверка, что номер порта действителен.
	if _, err := strconv.Atoi(port); err != nil {
		log.Fatal(err)
	}

	// Запуск HTTP-сервера
	err = http.ListenAndServe(":"+port, mux)
	if err != nil {
		log.Fatal(err)
	}
}
