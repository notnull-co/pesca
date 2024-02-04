package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/notnull-co/pesca/internal/channel"
	"github.com/notnull-co/pesca/internal/config"
)

type rest struct {
}

func New() channel.Channel {
	return &rest{}
}

func (r *rest) Start() error {
	e := echo.New()
	e.Use(middleware.Recover())

	e.GET("/", hello)

	return e.Start(":" + config.Get().Rest.Port)
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}
