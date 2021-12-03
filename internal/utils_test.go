package internal

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"scheduler/pkg/db"
	"testing"
	"time"
)

var myDB, err = sql.Open("mysql", "root:123456@/schedule?charset=utf8&parseTime=true")

func TestGetScheduleOne(t *testing.T) {
	var MinId int
	row := myDB.QueryRow("SELECT MIN(id) FROM schedules")
	err = row.Scan(&MinId)
	require.NoError(t, err)

	config := Config{
		Host: "localhost",
		Port: "9567",
	}
	result, err := GetScheduleOne(config, MinId)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	got := result.Data.CommandID
	var want int32
	row = myDB.QueryRow("SELECT command_id FROM schedules where id = ?", MinId)
	err = row.Scan(&want)
	require.NoError(t, err)
	require.Equal(t, want, got)

}

func TestPrepareScheduleStoreInCache(t *testing.T) {
	var MaxId int
	row := myDB.QueryRow("SELECT MAX(id) FROM schedules where end_date>now()")
	err = row.Scan(&MaxId)
	require.NoError(t, err)
	config := LoadConfig("../config")
	err = PrepareAllScheduleStoreInCache(config)
	require.NoError(t, err)

	result, err := GetScheduleOne(config, MaxId)
	require.NoError(t, err)
	require.NotEmpty(t, result)
	want := result.Data.CommandID

	cacheResult, err := GC.Get(int32(MaxId))
	require.NoError(t, err)
	require.NotEmpty(t, cacheResult)
	cacheResultStruct := cacheResult.(ScheduleOne)
	got := cacheResultStruct.CommandID
	require.Equal(t, want, got)

}

func TestPrepareAllCommandsStoreInCache(t *testing.T) {
	config := LoadConfig("../config")
	err = PrepareAllCommandsStoreInCache(config)
	require.NoError(t, err)

	var MaxId int
	row := myDB.QueryRow("SELECT MAX(id) FROM commands")
	err = row.Scan(&MaxId)
	require.NoError(t, err)
	var want string
	rows, err := myDB.Query("SELECT command FROM commands where id=?", MaxId)
	require.NoError(t, err)
	for rows.Next() {
		err = rows.Scan(&want)
		require.NoError(t, err)
	}
	cacheCommands, err := GC.Get("command")
	commandList, ok := cacheCommands.([]db.Command)
	require.True(t, ok)
	var got string
	for _, v := range commandList {
		if v.ID == int32(MaxId) {
			got = v.Command
		}
	}
	require.NoError(t, err)
	require.Equal(t, want, got)

}

func TestGocronRealAllCachedSchedule(t *testing.T) {
	config := LoadConfig("../config")
	err = PrepareAllCommandsStoreInCache(config)
	require.NoError(t, err)
	err = PrepareAllScheduleStoreInCache(config)
	require.NoError(t, err)

	err = GocronRealAllCachedSchedule()
	require.NoError(t, err)
	time.Sleep(1 * time.Second)

}
