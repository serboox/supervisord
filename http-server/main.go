package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

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

	router := setupRouter()

	log.Infof("HTTP-Server: Start in %s:%d ", cnf.Host, cnf.Port)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", cnf.Host, cnf.Port), router)

	if err != nil {
		log.Fatalf("Fail HTTP server start: %v", err)
	}
}

func setupRouter() (r *gin.Engine) {
	r = gin.New()
	r.GET("/healthcheck", healthCheck)
	r.NoRoute(noRoute)

	return
}

func healthCheck(c *gin.Context) {
	c.String(http.StatusOK, http.StatusText(http.StatusOK))
}

func noRoute(c *gin.Context) {
	c.String(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
}
