package internal

import (
	"scheduler/pkg/db"
)

type RequestId struct {
	Id int32 `json:"id"`
}

type Config struct {
	Host string `mapstructure:"SCHEDULE_CRUD_HOST"`
	Port string `mapstructure:"SCHEDULE_CRUD_PORT"`
}

type Result struct {
	Result string        `json:"result"`
	Error  string        `json:"error"`
	Data   []db.Schedule `json:"data"`
}

type SubResult struct {
	Result string       `json:"result"`
	Error  string       `json:"error"`
	Data   db.SubResult `json:"data"`
}

type ScheduleOneResult struct {
	Result string      `json:"result"`
	Error  string      `json:"error"`
	Data   ScheduleOne `json:"data"`
}

type ScheduleOne struct {
	db.Schedule
	RepeatWeekday []string `json:"repeat_weekday" binding:"required"`
	RepeatDay     []string `json:"repeat_day" binding:"required"`
	RepeatMonth   []string `json:"repeat_month" binding:"required"`
}

type AllCommandResponse struct {
	Result string       `json:"result"`
	Error  string       `json:"error"`
	Data   []db.Command `json:"data"`
}
