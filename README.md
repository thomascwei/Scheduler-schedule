# schedule-scheduler

```shell
docker build -t thomaswei/schedule-scheduler . --no-cache
# 加入環境變量從container訪問host db
docker run -d --name schedule-scheduler -p 9568:9568 -e SCHEDULE_CRUD_HOST=host.docker.internal thomaswei/schedule-scheduler 
```