package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	DAEMON = "daemon"
)

var (
	port           int
	damaen         bool
	connectTimeout int
	readTimeout    int
	reqTimeout     int
	proxy          *httputil.ReverseProxy
)

// Инициализация параметров
func init() {
	flag.IntVar(&port, "port", 8080, "Сетевой порт, на котором слушает прокси")
	flag.BoolVar(&damaen, DAEMON, false, "Запуск в фоне (демон)")
	flag.IntVar(&connectTimeout, "connect-timeout", 5, "Таймаут на установку TCP-соединения (в секундах)")
	flag.IntVar(&readTimeout, "read-timeout", 10, "Таймаут на чтение заголовка ответа (в секундах)")
	flag.IntVar(&reqTimeout, "request-timeout", 30, "Таймаут на выполнение запроса (контекст) (в секундах)")
}

// Обработчик проксирования
func ReverseProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[*] Receive a request from %s, request header: %v\n", r.RemoteAddr, r.Header)
	// Создаём контекст с ограничением по времени
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(reqTimeout)*time.Second)
	defer cancel()

	// Обновляем контекст в запросе
	r = r.WithContext(ctx)

	// Передаём запрос в уже сконфигурированный прокси
	proxy.ServeHTTP(w, r)

	log.Printf("[*] Receive the destination website response header: %v\n", w.Header())
}

// Удаление флага из аргументов
func StripSlice(slice []string, element string) []string {
	for i := 0; i < len(slice); {
		if slice[i] == element && i != len(slice)-1 {
			slice = append(slice[:i], slice[i+1:]...)
		} else if slice[i] == element && i == len(slice)-1 {
			slice = slice[:i]
		} else {
			i++
		}
	}
	return slice
}

// Запуск нового процесса (демон)
func SubProcess(args []string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		log.Printf("[-] Error: %s\n", err)
	}
	return cmd
}

func main() {
	flag.Parse()
	log.Printf("[*] PID: %d PPID: %d ARG: %v\n", os.Getpid(), os.Getppid(), os.Args)

	// Если нужно запустить в фоне
	if damaen {
		SubProcess(StripSlice(os.Args, "-"+DAEMON))
		log.Printf("[*] Daemon running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
		os.Exit(0)
	}

	// Создаём кастомный транспорт с таймаутами
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: time.Duration(connectTimeout) * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: time.Duration(readTimeout) * time.Second,
		TLSHandshakeTimeout:   5 * time.Second, // Можно тоже вынести в флаг, если нужно
		// Другие настройки по необходимости
	}

	// Настраиваем `ReverseProxy`
	target := "api.openai.com"
	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.URL.Host = target
		req.Host = target
	}
	proxy = &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}

	log.Printf("[*] Forever running in PID: %d PPID: %d\n", os.Getpid(), os.Getppid())
	log.Printf("[*] Starting server at port %v\n", port)

	// Запускаем сервер
	if err := http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(ReverseProxyHandler)); err != nil {
		log.Fatal(err)
	}
}
