package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Ticket struct {
	ID           string
	OwnerName    string
	BirthDate    string
	StartDate    string
	StartTime    string
	EndDate      string
	EndTime      string
	Coverage     string
	TicketClass  string
	TicketNumber string
	QRCodePath   string
}

var (
	templates = template.Must(template.ParseGlob("templates/*.html"))
	db        *sql.DB
)

func main() {
	os.MkdirAll("static/uploads", 0755)

	// Получаем путь к базе данных из переменной окружения или используем значение по умолчанию
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./tickets.db"
	}

	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id TEXT PRIMARY KEY,
			owner_name TEXT NOT NULL,
			birth_date TEXT NOT NULL,
			start_date TEXT NOT NULL,
			start_time TEXT NOT NULL,
			end_date TEXT NOT NULL,
			end_time TEXT NOT NULL,
			coverage TEXT NOT NULL,
			ticket_class TEXT NOT NULL,
			ticket_number TEXT NOT NULL,
			qr_code_path TEXT NOT NULL
		)
	`)
	if err != nil {
		log.Fatalf("Ошибка создания таблицы: %v", err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/create-ticket", createTicketHandler)
	http.HandleFunc("/save-ticket", saveTicketHandler)
	http.HandleFunc("/ticket/", viewTicketHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Сервер запущен на http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	templates.ExecuteTemplate(w, "index.html", nil)
}

func createTicketHandler(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "create-ticket.html", nil)
}

func saveTicketHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Метод не разрешен", http.StatusMethodNotAllowed)
		return
	}

	r.ParseMultipartForm(10 << 20)

	token := r.FormValue("token")
	if token != "sadadaddad" {
		http.Error(w, "Неправильный токен", http.StatusUnauthorized)
		return
	}
	ticketID := fmt.Sprintf("%d", time.Now().UnixNano())
	ownerName := r.FormValue("ownerName")
	birthDate := r.FormValue("birthDate")
	startDate := r.FormValue("startDate")
	startTime := r.FormValue("startTime")
	endDate := r.FormValue("endDate")
	endTime := r.FormValue("endTime")
	coverage := r.FormValue("coverage")
	ticketClass := r.FormValue("ticketClass")
	ticketNumber := r.FormValue("ticketNumber")

	file, handler, err := r.FormFile("qrCode")
	if err != nil {
		http.Error(w, "Ошибка при загрузке QR-кода", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%s%s", ticketID, filepath.Ext(handler.Filename))
	filePath := filepath.Join("static/uploads", filename)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Ошибка при сохранении файла", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Ошибка при сохранении файла", http.StatusInternalServerError)
		return
	}

	qrCodePath := "/static/uploads/" + filename

	_, err = db.Exec(`
		INSERT INTO tickets (
			id, owner_name, birth_date, start_date, start_time, 
			end_date, end_time, coverage, ticket_class, ticket_number, qr_code_path
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		ticketID, ownerName, birthDate, startDate, startTime,
		endDate, endTime, coverage, ticketClass, ticketNumber, qrCodePath)

	if err != nil {
		http.Error(w, "Ошибка при сохранении данных", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/ticket/"+ticketID, http.StatusSeeOther)
}

func viewTicketHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	ticketID := parts[2]

	var ticket Ticket
	err := db.QueryRow(`
		SELECT id, owner_name, birth_date, start_date, start_time, 
		       end_date, end_time, coverage, ticket_class, ticket_number, qr_code_path
		FROM tickets WHERE id = ?
	`, ticketID).Scan(
		&ticket.ID, &ticket.OwnerName, &ticket.BirthDate, &ticket.StartDate, &ticket.StartTime,
		&ticket.EndDate, &ticket.EndTime, &ticket.Coverage, &ticket.TicketClass, &ticket.TicketNumber, &ticket.QRCodePath,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
		} else {
			http.Error(w, "Ошибка при получении данных", http.StatusInternalServerError)
		}
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	shareURL := fmt.Sprintf("%s://%s/ticket/%s", scheme, host, ticketID)

	data := struct {
		Ticket   Ticket
		ShareURL string
	}{
		Ticket:   ticket,
		ShareURL: shareURL,
	}

	templates.ExecuteTemplate(w, "view-ticket.html", data)
}
