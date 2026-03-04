package httpapp

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lab5-series-tracker/internal/storage"
)

type Server struct {
	Addr string
	DB   *sql.DB
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	fmt.Println("Servidor corriendo en http://localhost" + s.Addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	raw, err := readFullRequest(conn)
	if err != nil {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "bad request")
		return
	}

	method, path, headers, body := parseRequest(raw)

	// Static files
	if path == "/favicon.ico" && method == "GET" {
		s.serveStatic(conn, "/static/favicon.ico")
		return
	}
	if strings.HasPrefix(path, "/static/") && method == "GET" {
		s.serveStatic(conn, path)
		return
	}

	// Rutas del lab
	if path == "/" && method == "GET" {
		s.handleIndex(conn)
		return
	}

	if path == "/create" && method == "GET" {
		s.handleCreateForm(conn)
		return
	}

	if path == "/create" && method == "POST" {
		s.handleCreatePost(conn, headers, body)
		return
	}

	if strings.HasPrefix(path, "/update") && method == "POST" {
		s.handleUpdate(conn, path)
		return
	}

	writeResponse(conn, 404, "text/plain; charset=utf-8", "not found")
}

/* ---------------- Handlers ---------------- */

func (s *Server) handleIndex(conn net.Conn) {
	series, err := storage.ListSeries(s.DB)
	if err != nil {
		writeResponse(conn, 500, "text/plain; charset=utf-8", "db error: "+err.Error())
		return
	}

	tpl, err := template.ParseFiles("web/templates/index.html")
	if err != nil {
		writeResponse(conn, 500, "text/plain; charset=utf-8", "template error: "+err.Error())
		return
	}

	var b strings.Builder
	if err := tpl.Execute(&b, series); err != nil {
		writeResponse(conn, 500, "text/plain; charset=utf-8", "render error: "+err.Error())
		return
	}

	writeResponse(conn, 200, "text/html; charset=utf-8", b.String())
}

func (s *Server) handleCreateForm(conn net.Conn) {
	tpl, err := template.ParseFiles("web/templates/create.html")
	if err != nil {
		writeResponse(conn, 500, "text/plain; charset=utf-8", "template error: "+err.Error())
		return
	}
	var b strings.Builder
	_ = tpl.Execute(&b, nil)
	writeResponse(conn, 200, "text/html; charset=utf-8", b.String())
}

func (s *Server) handleCreatePost(conn net.Conn, headers map[string]string, body string) {
	_ = headers // por si luego quieres validar content-type

	values, err := url.ParseQuery(body)
	if err != nil {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "no pude parsear el body")
		return
	}

	name := values.Get("series_name")
	currentStr := values.Get("current_episode")
	totalStr := values.Get("total_episodes")

	currentEp, _ := strconv.Atoi(currentStr)
	totalEp, _ := strconv.Atoi(totalStr)

	
	if err := storage.ValidateSerie(name, currentEp, totalEp); err != nil {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "validación: "+err.Error())
		return
	}

	
	if err := storage.InsertSerie(s.DB, name, currentEp, totalEp); err != nil {
		writeResponse(conn, 500, "text/plain; charset=utf-8", "db error: "+err.Error())
		return
	}

	
	writeRedirect303(conn, "/")
}

func (s *Server) handleUpdate(conn net.Conn, path string) {
	// /update?id=3
	parts := strings.SplitN(path, "?", 2)
	if len(parts) < 2 {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "falta query id")
		return
	}

	params, err := url.ParseQuery(parts[1])
	if err != nil {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "query inválida")
		return
	}

	idStr := params.Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		writeResponse(conn, 400, "text/plain; charset=utf-8", "id inválido")
		return
	}


	_ = storage.IncrementEpisode(s.DB, id)

	// Para fetch() devolvemos “ok”
	writeResponse(conn, 200, "text/plain; charset=utf-8", "ok")
}

func (s *Server) serveStatic(conn net.Conn, path string) {
	rel := strings.TrimPrefix(path, "/static/")
	filePath := filepath.Join("web/static", rel)

	data, err := os.ReadFile(filePath)
	if err != nil {
		writeResponse(conn, 404, "text/plain; charset=utf-8", "static not found")
		return
	}

	writeResponseBytes(conn, 200, contentType(filePath), data)
}

/* ---------------- HTTP helpers ---------------- */

func readFullRequest(conn net.Conn) (string, error) {
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	req := string(buf[:n])

	for !strings.Contains(req, "\r\n\r\n") {
		n, err = conn.Read(buf)
		if err != nil {
			return "", err
		}
		req += string(buf[:n])
	}

	headersPart := req[:strings.Index(req, "\r\n\r\n")]
	contentLength := 0
	for _, line := range strings.Split(headersPart, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			v := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
			contentLength, _ = strconv.Atoi(v)
		}
	}

	bodyStart := strings.Index(req, "\r\n\r\n") + 4
	currentBody := req[bodyStart:]

	for len(currentBody) < contentLength {
		n, err = conn.Read(buf)
		if err != nil {
			return "", err
		}
		currentBody += string(buf[:n])
	}

	return req, nil
}

func parseRequest(raw string) (method string, path string, headers map[string]string, body string) {
	headers = make(map[string]string)

	parts := strings.SplitN(raw, "\r\n\r\n", 2)
	head := parts[0]
	if len(parts) == 2 {
		body = parts[1]
	}

	lines := strings.Split(head, "\r\n")
	if len(lines) == 0 {
		return
	}

	first := strings.Split(lines[0], " ")
	if len(first) >= 2 {
		method = first[0]
		path = first[1]
	}

	for _, line := range lines[1:] {
		if strings.Contains(line, ":") {
			p := strings.SplitN(line, ":", 2)
			headers[strings.ToLower(strings.TrimSpace(p[0]))] = strings.TrimSpace(p[1])
		}
	}

	return
}

func writeResponse(conn net.Conn, status int, contentType string, body string) {
	msg := fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, statusText(status))
	msg += "Content-Type: " + contentType + "\r\n"
	msg += fmt.Sprintf("Content-Length: %d\r\n", len([]byte(body)))
	msg += "Connection: close\r\n\r\n"
	msg += body
	_, _ = io.WriteString(conn, msg)
}

func writeResponseBytes(conn net.Conn, status int, contentType string, body []byte) {
	head := fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, statusText(status))
	head += "Content-Type: " + contentType + "\r\n"
	head += fmt.Sprintf("Content-Length: %d\r\n", len(body))
	head += "Connection: close\r\n\r\n"
	_, _ = conn.Write([]byte(head))
	_, _ = conn.Write(body)
}

func writeRedirect303(conn net.Conn, location string) {
	msg := "HTTP/1.1 303 See Other\r\n"
	msg += "Location: " + location + "\r\n"
	msg += "Content-Length: 0\r\n"
	msg += "Connection: close\r\n\r\n"
	_, _ = io.WriteString(conn, msg)
}

func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 303:
		return "See Other"
	case 400:
		return "Bad Request"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	default:
		return "OK"
	}
}

func contentType(path string) string {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(low, ".js"):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(low, ".ico"):
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
