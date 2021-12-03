package main

import (
	"github.com/gin-gonic/gin"
	"github.com/jasonlvhit/gocron"
	"io"
	"log"
	"os"
	"path/filepath"
	"scheduler/internal"
)

var (
	Trace *log.Logger
	Info  *log.Logger
	Error *log.Logger
)

// 在初始化程序中將所有排程存進cache
func init() {
	// log配置
	newPath := filepath.Join(".", "log")
	_ = os.MkdirAll(newPath, os.ModePerm)
	file, err := os.OpenFile("./log/main.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("can not open log file: " + err.Error())
	}
	Trace = log.New(os.Stdout, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(io.MultiWriter(file, os.Stdout), "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(io.MultiWriter(file, os.Stdout), "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	//
	err = internal.PrepareAllScheduleStoreInCache(internal.CRUDconfig)
	if err != nil {
		log.Fatal("initial schedule cache fail: " + err.Error())
	}
	//
	err = internal.PrepareAllCommandsStoreInCache(internal.CRUDconfig)
	if err != nil {
		log.Fatal("initial commands cache fail: " + err.Error())
	}

}
func main() {

	go func() {
		gocron.Every(1).Second().Do(internal.GocronRealAllCachedSchedule)
		<-gocron.Start()
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.POST("/schedule/V1/update_cached_schedule", internal.UpdateScheduleRoute)
	r.GET("/schedule/V1/update_cached_commands", internal.UpdateAllCommandsRoute)
	r.Run(":9568")
}
