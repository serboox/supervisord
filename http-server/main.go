package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
)

var healthCheckStatus int64 = 200

type config struct {
	Host      string
	Port      int
	DebugMode bool
}

func main() {
	cnf := config{
		Host:      "127.0.0.1",
		Port:      8077,
		DebugMode: true,
	}

	ginMode := gin.ReleaseMode
	if cnf.DebugMode {
		ginMode = gin.DebugMode
	}

	gin.SetMode(ginMode)
	log.SetLevel(log.DebugLevel)

	initSignals()

	router := setupRouter()

	log.Infof("HTTP-Server: Start in %s:%d ", cnf.Host, cnf.Port)
	log.Infof("HTTP-Server: PID %d", os.Getpid())

	err := http.ListenAndServe(fmt.Sprintf("%s:%d", cnf.Host, cnf.Port), router)

	if err != nil {
		log.Fatalf("Fail HTTP server start: %v", err)
	}
}

func setupRouter() (r *gin.Engine) {
	r = gin.New()
	r.GET("/healthcheck", healthCheck)
	r.NoRoute(noRoute)

	r.Use(logger())

	return
}

func healthCheck(c *gin.Context) {
	if healthCheckStatus != http.StatusOK {
		c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	c.String(http.StatusOK, http.StatusText(http.StatusOK))
}

func noRoute(c *gin.Context) {
	c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}

func initSignals() {
	sigusr1 := make(chan os.Signal, 1)
	signal.Notify(sigusr1, syscall.SIGUSR1) // 10

	go func() {
		sig := <-sigusr1

		log.WithFields(log.Fields{"signal": sig}).Info("receive a signal SIGUSR1")
		atomic.StoreInt64(&healthCheckStatus, http.StatusOK)
	}()

	sigusr2 := make(chan os.Signal, 1)
	signal.Notify(sigusr2, syscall.SIGUSR2) // 12

	go func() {
		sig := <-sigusr2

		log.WithFields(log.Fields{"signal": sig}).Info("receive a signal SIGUSR2")
		atomic.StoreInt64(&healthCheckStatus, http.StatusInternalServerError)
	}()
}

func logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		uuidType, err := uuid.NewV4()
		if err != nil {
			log.Error(err)
		}

		requestID := uuidType.String()

		c.Set("request_uuid", requestID)
		c.Set("logger", log.WithField("request_uuid", requestID))

		start := time.Now().UTC()

		c.Next()

		end := time.Now().UTC()
		latency := end.Sub(start)

		var fields log.Fields = make(map[string]interface{})
		fields["date"] = start.Format("2006-01-02")
		period := fmt.Sprintf(
			"(%s)->(%s)",
			start.Format("15:04:05.999"),
			end.Format("15:04:05.999"),
		)
		fields["time"] = period
		fields["uuid"] = requestID

		e, exists := c.Get("api_error")
		if exists {
			fields["error"] = e.(error).Error()
		}

		ginErr := c.Errors.String()
		if ginErr != "" {
			log.Error(ginErr)
		}

		log.WithFields(fields).Infof(
			"GIN: %s %s %s code=%d ip=%s",
			c.Request.Method,
			c.Request.URL.Path,
			fmt.Sprint(latency),
			c.Writer.Status(),
			c.ClientIP(),
		)
	}
}
