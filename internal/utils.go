package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/jinzhu/copier"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"scheduler/pkg/db"
	"scheduler/pkg/pb"
	"strconv"
	"strings"
	"time"
)

var (
	file, _ = os.OpenFile("./log/utils.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	Trace   = log.New(os.Stdout, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	Info    = log.New(io.MultiWriter(file, os.Stdout), "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error   = log.New(io.MultiWriter(file, os.Stdout), "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	GC         = gcache.New(200).Build()
	CRUDconfig = LoadConfig("./config")
)

// 字串slice是否包含指定值
func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

// 讀專案中的config檔
func LoadConfig(mypath string) (config Config) {
	viper.AddConfigPath(mypath)
	// 為了讓執行test也能讀到config添加索引路徑
	wd, err := os.Getwd()
	parent := filepath.Dir(wd)
	viper.AddConfigPath(path.Join(parent, mypath))
	viper.SetConfigName("app")
	viper.SetConfigType("yaml")
	// 若有同名環境變量則使用環境變量
	viper.AutomaticEnv()
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatal("can not load config: " + err.Error())
	}
	err = viper.Unmarshal(&config)
	if err != nil {
		log.Fatal("can not load config: " + err.Error())
	}
	return
}

// 將所有排程存進cache
func PrepareAllScheduleStoreInCache(config Config) error {
	current := time.Now()
	// 從API取得數據
	MainUrl := fmt.Sprintf("http://%v:%v/schedule/V1/all_schedules", config.Host, config.Port)
	mainSchedule, err := GetAllMainSchedule(MainUrl)
	if err != nil {
		return err
	}
	if mainSchedule.Result != "ok" {
		Error.Println(mainSchedule.Error)
		return errors.New(mainSchedule.Error)
	}
	// 從API取得數據
	SubUrl := fmt.Sprintf("http://%v:%v/schedule/V1/get_all_sub_schedules", config.Host, config.Port)
	subSchedule, err := GetAllSubSchedules(SubUrl)
	if err != nil {
		return err
	}
	if subSchedule.Result != "ok" {
		Error.Println(subSchedule.Error)
		return errors.New(subSchedule.Error)
	}

	for _, schedule := range mainSchedule.Data {
		ScheduleOne := ScheduleOne{}
		ScheduleOne.Schedule = schedule
		//如果到期時間早於當前時間則跳過, GO的continue是pass, 並非跳出迴圈(與Python不同)
		if current.Sub(schedule.EndDate) > 0 {
			continue
		}
		for _, repeat := range subSchedule.Data.RepeatWeekday {
			if repeat.ScheduleID == schedule.ID {
				ScheduleOne.RepeatWeekday = append(ScheduleOne.RepeatWeekday, repeat.Weekday)
			}
		}
		for _, repeat := range subSchedule.Data.RepeatMonth {
			if repeat.ScheduleID == schedule.ID {
				ScheduleOne.RepeatMonth = append(ScheduleOne.RepeatMonth, repeat.Month)
			}
		}
		for _, repeat := range subSchedule.Data.RepeatDay {
			if repeat.ScheduleID == schedule.ID {
				ScheduleOne.RepeatDay = append(ScheduleOne.RepeatMonth, repeat.Day)
			}
		}
		timeDiff := ScheduleOne.EndDate.Sub(current)
		// 將排程寫進cache並設置到期時間
		err = GC.SetWithExpire(ScheduleOne.ID, ScheduleOne, timeDiff)

		if err != nil {
			Error.Println(err)
			return err
		}
	}
	Info.Println("finish schedule cache init")
	return nil
}

func GetAllMainSchedule(url string) (result Result, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Error.Println(err)
		return
	}
	res, err := client.Do(req)
	if err != nil {
		Error.Println(err)
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		Error.Println(err)
		return
	}
	//var result Result
	err = json.Unmarshal(body, &result)
	if err != nil {
		Error.Println(err)
		return
	}
	if result.Result != "ok" {
		Error.Println(result.Error)
		return
	}
	return
}

func GetAllSubSchedules(url string) (result SubResult, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Error.Println(err)
		return
	}
	res, err := client.Do(req)
	if err != nil {
		Error.Println(err)
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		Error.Println(err)
		return
	}
	err = json.Unmarshal(body, &result)
	if err != nil {
		Error.Println(err)
		return
	}
	if result.Result != "ok" {
		Error.Println(result.Error)
		return
	}
	return
}

func GetScheduleOne(config Config, ID int) (result ScheduleOneResult, err error) {
	url := fmt.Sprintf("http://%v:%v/schedule/V1/get_one_schedule", config.Host, config.Port)
	requestBody, err := json.Marshal(map[string]interface{}{
		"id": ID,
	})
	if err != nil {
		Error.Println(err)
		return
	}
	response, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		Error.Println(err)
		return
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	err = json.Unmarshal(body, &result)
	if err != nil {
		Error.Println(err)
		return
	}
	return
}
func GRPCGetScheduleOne(config Config, ID int32) (result ScheduleOneResult, err error) {
	address := fmt.Sprintf("%v:%v", config.Host, config.GRPCPort)
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		Error.Println(err)
		result.Error = err.Error()
		return
	}
	defer conn.Close()
	c := pb.NewGetScheduleCRUDServiceClient(conn) // Creates a new instance of a hello client, defined in the protocol buffer file
	resp, err := c.GetScheduleOne(context.Background(), &pb.GetScheduleOneReq{Id: ID})
	if err != nil {
		Error.Println(err)
		result.Error = err.Error()
		return
	}
	err = copier.Copy(&result.Data.Schedule, resp)
	if err != nil {
		Error.Println(err)
		result.Error = err.Error()
		return
	}
	// copier僅能複製名稱與型態相同的field, 所以不能複製的要個別賦予
	result.Data.ID = resp.Id
	result.Data.TimeTypeID = resp.TimeTypeId
	result.Data.CommandID = resp.CommandId
	result.Data.StartDate = resp.StartDate.AsTime()
	result.Data.EndDate = resp.EndDate.AsTime()
	result.Data.CreateTime = resp.CreateTime.AsTime()
	result.Result = "ok"
	return
}

func GetALlCommands(url string) (result []db.Command, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		Error.Println(err)
		return
	}
	res, err := client.Do(req)
	if err != nil {
		Error.Println(err)
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		Error.Println(err)
		return
	}
	AllCommandResponse := AllCommandResponse{}
	err = json.Unmarshal(body, &AllCommandResponse)
	if err != nil {
		Error.Println(err)
		return
	}
	if AllCommandResponse.Result != "ok" {
		Error.Println(AllCommandResponse.Error)
		return
	}
	result = AllCommandResponse.Data
	return
}

// 將所有command存進cache
func PrepareAllCommandsStoreInCache(config Config) (err error) {
	// 從API取得數據
	url := fmt.Sprintf("http://%v:%v/schedule/V1/all_commands", config.Host, config.Port)
	allCommands, err := GetALlCommands(url)
	if err != nil {
		return
	}
	allCommandsWithoutDate := make([]cacheCommand, 0)
	for _, v := range allCommands {
		singleCommand := cacheCommand{}
		singleCommand.ID = v.ID
		singleCommand.Command = v.Command
		allCommandsWithoutDate = append(allCommandsWithoutDate, singleCommand)
	}
	err = GC.Set("command", allCommandsWithoutDate)
	if err != nil {
		return
	}
	Info.Println("finish commands cache init")
	return
}

// 將所有command存進cache, gRPC版
func GPRCPrepareAllCommandsStoreInCache(config Config) (err error) {
	address := fmt.Sprintf("%v:%v", config.Host, config.GRPCPort)
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		Error.Println(err)
		return
	}
	defer conn.Close()
	c := pb.NewGetScheduleCRUDServiceClient(conn) // Creates a new instance of a hello client, defined in the protocol buffer file
	resp, err := c.GetCommands(context.Background(), &pb.Empty{})
	if err != nil {
		Error.Println(err)
		return
	}
	allCommands := make([]cacheCommand, 0)
	Trace.Printf("+%v\n", resp)
	for _, v := range resp.Command {
		Trace.Printf("+%v\n", v)
		oneCommand := cacheCommand{}
		oneCommand.ID = v.Id
		oneCommand.Command = v.Command
		allCommands = append(allCommands, oneCommand)
	}
	Trace.Println("")

	err = GC.Set("command", allCommands)
	if err != nil {
		return
	}
	Info.Println("finish commands cache init")
	return
}

// 讀取所有cache裡的schedule後逐一開goroutine檢查
func GocronRealAllCachedSchedule(closing chan struct{}) {
	for {
		select {
		case <-closing:
			Info.Println("closing")
			return
		default:
			current := time.Now().Round(time.Second)
			Trace.Println(current)
			schedules := GC.GetALL(true)
			delete(schedules, "command")

			for _, s := range schedules {
				ss, ok := s.(ScheduleOne)
				if !ok {
					Error.Println("type assert error")
					return
				}
				if ss.Enable {
					go GoroutineScheduleTriggerCheck(closing, ss, current)
				}
			}
			return
		}
	}

}

// 檢查schedule規則套用當前時間是否會觸發
func GoroutineScheduleTriggerCheck(close chan struct{}, schedule ScheduleOne, BaseTime time.Time) {
	//Trace.Println(schedule)
	for {
		select {
		case <-close:
			Info.Println("closing")
			return
		default:
			StartYear, StartMonth, StartDay := schedule.StartDate.Date()
			BaseYear, BaseMonth, BaseDay := BaseTime.Date()
			switch schedule.TimeTypeID {
			// 只執行一次, 計算出唯一的執行時間比對符合後觸發
			case 0:
				HHMMSS := strings.Split(schedule.AtTime, ":")
				HH, err := strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err := strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err := strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}

				targetTime := time.Date(StartYear, StartMonth, StartDay, HH, MM, SS, 0, time.Local)
				timeDiff := targetTime.Sub(BaseTime)
				//Trace.Println(targetTime, current, timeDiff)
				if timeDiff == 0 {
					_ = GetCommandFromCacheAndPublish(int(schedule.ID), schedule.CommandID)
				}
				Trace.Println("schedule", schedule.ID, "非指定時間,直接結束")
			// 指定日期區間內->每天(間隔?天)->指定時間區間內->指定間隔秒數
			case 1:
				// 未達開始日期,直接結束
				if schedule.StartDate.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始日期,直接結束")
					return
				}
				// 超過結束日期,直接結束
				if schedule.EndDate.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束日期,直接結束")
					return
				}
				// 未達開始時間,直接結束
				HHMMSS := strings.Split(schedule.StartTime, ":")
				HH, err := strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err := strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err := strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				StartTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if StartTime.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始時間,直接結束")
					return
				}
				// 超過結束時間,直接結束
				HHMMSS = strings.Split(schedule.EndTime, ":")
				HH, err = strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err = strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err = strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				EndTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if EndTime.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束時間,直接結束")
					return
				}
				// 判斷是否符合日期間隔,不符合直接結束
				if schedule.IntervalDay > 1 {
					CurrentDate := time.Date(BaseYear, BaseMonth, BaseDay, 0, 0, 0, 0, time.Local)
					StartDate := time.Date(StartYear, StartMonth, StartDay, 0, 0, 0, 0, time.Local)
					if int32(CurrentDate.Sub(StartDate).Hours())%(schedule.IntervalDay*24) != 0 {
						Trace.Println("schedule", schedule.ID, "不符合日期間隔,直接結束")
						return
					}
				}
				// 判斷是否符合時間(秒數)間隔,不符合直接結束
				if schedule.IntervalSeconds > 1 {
					if int32(BaseTime.Sub(StartTime).Seconds())%(schedule.IntervalSeconds) != 0 {
						Trace.Println("schedule", schedule.ID, "不符合秒數間隔,直接結束")
						return
					}
				}
				_ = GetCommandFromCacheAndPublish(int(schedule.ID), schedule.CommandID)
			// 指定日期區間內->指定時間區間內->指定的星期幾->是否重複->是,指定間隔秒數
			//											    ->否,判斷是否等於at time
			case 2:
				// 未達開始日期,直接結束
				if schedule.StartDate.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始日期,直接結束")
					return
				}
				// 超過結束日期,直接結束
				if schedule.EndDate.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束日期,直接結束")
					return
				}
				// 未達開始時間,直接結束
				HHMMSS := strings.Split(schedule.StartTime, ":")
				HH, err := strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err := strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err := strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				StartTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if StartTime.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始時間,直接結束")
					return
				}
				// 超過結束時間,直接結束
				HHMMSS = strings.Split(schedule.EndTime, ":")
				HH, err = strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err = strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err = strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				EndTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if EndTime.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束時間,直接結束")
					return
				}
				// 不在指定的星期幾內,直接結束
				if !contains(schedule.RepeatWeekday, BaseTime.Weekday().String()) {
					Trace.Println("schedule", schedule.ID, "不在指定的星期幾內,直接結束")
					return
				}
				// 是否重複->是
				if schedule.Repeat {
					// 判斷是否符合時間(秒數)間隔,不符合直接結束
					if schedule.IntervalSeconds > 1 {
						if int32(BaseTime.Sub(StartTime).Seconds())%(schedule.IntervalSeconds) != 0 {
							Trace.Println("schedule", schedule.ID, "不符合秒數間隔,直接結束")
							return
						}
					}
				} else {
					// 是否重複->否
					HHMMSS = strings.Split(schedule.AtTime, ":")
					HH, err = strconv.Atoi(HHMMSS[0])
					if err != nil {
						Error.Println(err)
						return
					}
					MM, err = strconv.Atoi(HHMMSS[1])
					if err != nil {
						Error.Println(err)
						return
					}
					SS, err = strconv.Atoi(HHMMSS[2])
					if err != nil {
						Error.Println(err)
						return
					}

					targetTime := time.Date(BaseTime.Year(), BaseTime.Month(), BaseTime.Day(), HH, MM, SS, 0, time.Local)
					timeDiff := targetTime.Sub(BaseTime)
					//Trace.Println(targetTime, BaseTime, timeDiff)
					if timeDiff != 0 {
						Trace.Println("schedule", schedule.ID, "不為指定的時間(AtTime),直接結束")
						return
					}
				}

				_ = GetCommandFromCacheAndPublish(int(schedule.ID), schedule.CommandID)
			// 指定日期區間內->指定時間區間內->指定的月->指定的日->是否重複->是,指定間隔秒數
			//											         ->否,判斷是否等於at time
			case 3:
				// 未達開始日期,直接結束
				if schedule.StartDate.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始日期,直接結束")
					return
				}
				// 超過結束日期,直接結束
				if schedule.EndDate.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束日期,直接結束")
					return
				}
				// 未達開始時間,直接結束
				HHMMSS := strings.Split(schedule.StartTime, ":")
				HH, err := strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err := strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err := strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				StartTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if StartTime.Sub(BaseTime) > 0 {
					Trace.Println("schedule", schedule.ID, "未達開始時間,直接結束")
					return
				}
				// 超過結束時間,直接結束
				HHMMSS = strings.Split(schedule.EndTime, ":")
				HH, err = strconv.Atoi(HHMMSS[0])
				if err != nil {
					Error.Println(err)
					return
				}
				MM, err = strconv.Atoi(HHMMSS[1])
				if err != nil {
					Error.Println(err)
					return
				}
				SS, err = strconv.Atoi(HHMMSS[2])
				if err != nil {
					Error.Println(err)
					return
				}
				EndTime := time.Date(BaseYear, BaseMonth, BaseDay, HH, MM, SS, 0, time.Local)
				if EndTime.Sub(BaseTime) < 0 {
					Trace.Println("schedule", schedule.ID, "超過結束時間,直接結束")
					return
				}
				// 不在指定的月裡,直接結束
				if !contains(schedule.RepeatMonth, strconv.Itoa(int(BaseTime.Month()))) {
					Trace.Println("schedule", schedule.ID, "不在指定的月裡,直接結束")
					return
				}
				// 不在指定的日裡,直接結束
				if !contains(schedule.RepeatDay, strconv.Itoa(BaseTime.Day())) {
					Trace.Println("schedule", schedule.ID, "不在指定的日裡,直接結束")
					return
				}
				// 是否重複->是
				if schedule.Repeat {
					// 判斷是否符合時間(秒數)間隔,不符合直接結束
					if schedule.IntervalSeconds > 1 {
						if int32(BaseTime.Sub(StartTime).Seconds())%(schedule.IntervalSeconds) != 0 {
							Trace.Println("schedule", schedule.ID, "不符合秒數間隔,直接結束")
							return
						}
					}
				} else {
					// 是否重複->否
					HHMMSS = strings.Split(schedule.AtTime, ":")
					HH, err = strconv.Atoi(HHMMSS[0])
					if err != nil {
						Error.Println(err)
						return
					}
					MM, err = strconv.Atoi(HHMMSS[1])
					if err != nil {
						Error.Println(err)
						return
					}
					SS, err = strconv.Atoi(HHMMSS[2])
					if err != nil {
						Error.Println(err)
						return
					}

					targetTime := time.Date(BaseTime.Year(), BaseTime.Month(), BaseTime.Day(), HH, MM, SS, 0, time.Local)
					timeDiff := targetTime.Sub(BaseTime)
					if timeDiff != 0 {
						Trace.Println("schedule", schedule.ID, "不為指定的時間(AtTime),直接結束")
						return
					}
				}

				_ = GetCommandFromCacheAndPublish(int(schedule.ID), schedule.CommandID)

			default:
				Error.Println("schedule", schedule.ID, "Time TYPE ID error")
			}
			return
		}
	}

}

// schedule符合觸發條件取cache中的command字串後發送
func GetCommandFromCacheAndPublish(ScheduleID int, CommandID int32) error {
	commands, err := GC.Get("command")
	if err != nil {
		Error.Println(err)
		return err
	}
	commandList, ok := commands.([]cacheCommand)
	if !ok {
		Error.Println("type assert error")
		return errors.New("type assert error")
	}
	var targetCommand string
	for _, command := range commandList {
		if command.ID == CommandID {
			targetCommand = command.Command
		}
	}
	Trace.Println("ScheduleID:" + strconv.Itoa(ScheduleID) + "; Trigger :" + targetCommand)
	return nil
}

// schedule變更更新cache
func UpdateCacheSchedule(id int) (err error) {
	//result, err := GetScheduleOne(CRUDconfig, id)
	result, err := GRPCGetScheduleOne(CRUDconfig, int32(id))
	if err != nil {
		return
	}
	if result.Error != "" {
		return
	}
	// 開始檢查邏輯
	current := time.Now()
	timeDiff := result.Data.EndDate.Sub(current)
	//如果到期時間早於當前時間則結束
	if timeDiff < 0 {
		return
	}
	// 將排程寫進cache並設置到期時間
	err = GC.SetWithExpire(int32(id), result.Data, timeDiff)
	if err != nil {
		return
	}
	return
}
