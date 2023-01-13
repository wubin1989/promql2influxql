# promql2influxql
![Coverage](https://img.shields.io/badge/Coverage-83.3%25-brightgreen)
<span>
<a href="https://godoc.org/github.com/wubin1989/promql2influxql"><img src="https://godoc.org/github.com/wubin1989/promql2influxql?status.png" alt="GoDoc"></a>
<a href="https://github.com/wubin1989/promql2influxql/actions/workflows/go.yml"><img src="https://github.com/wubin1989/promql2influxql/actions/workflows/go.yml/badge.svg?branch=main" alt="Go"></a>
<a href="https://goreportcard.com/report/github.com/wubin1989/promql2influxql"><img src="https://goreportcard.com/badge/github.com/wubin1989/promql2influxql" alt="Go Report Card"></a>
<a href="https://github.com/wubin1989/promql2influxql"><img src="https://img.shields.io/github/v/release/wubin1989/promql2influxql?style=flat-square" alt="Release"></a>
<a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
</span>
<br/>

本项目是PromQL转InfluxQL转译器和适配器，实现了传入原生PromQL查询语句，转成InfluxQL语句，并查询InfluxDB数据库返回结果。

## 前置条件
本程序基于以下前置条件开发：
- 基础设施版本：
  - Prometheus v2.41.0：Docker镜像 [prom/prometheus:v2.41.0](https://hub.docker.com/r/prom/prometheus)
  - InfluxDB v1.8.10：Docker镜像 [influxdb:1.8.10](https://hub.docker.com/_/influxdb)
- Prometheus数据写入方式：
  - [Prometheus Remote Write机制](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
  - [InfluxDB的/api/v1/prom/write接口](https://docs.influxdata.com/influxdb/v1.8/supported_protocols/prometheus/)

## 项目状态
项目结构和核心代码基本稳定，后续开发以新增特性和性能优化为主，尽量兼容旧版本API。v1.0版本之前不建议用于生产环境。

## 特性说明
- 支持Prometheus四种指标类型：Counter、Gauge、Histogram和Summary。
- 支持PromQL的7种选择器表达式、10种聚合操作表达式、13种二元操作表达式、24种内置函数转译到InfluxQL查询语句。
- 支持作为Prometheus数据源的适配器服务接入Grafana，输入PromQL查询语句实际由适配器服务向InfluxDB实例发起查询请求和返回结果。
- 既可以作为第三方库在你的项目中依赖，也可以作为微服务单独部署。
- 面向微服务架构的代码组织结构，易扩展，如果需要新增其他数据源的转译器/适配器，只需在根路径下复制一套`promql`包的代码，修改使用即可。比如需要新增对Elasticsearch数据源的适配，只需将`promql`包的代码复制一套改成`elasticql`（或随便什么名字）包，在里面改就行。每个转译器代码包里都有一个适配层RESTful服务。

## 截图
截图中的dashboard来自[Go Metrics](https://grafana.com/grafana/dashboards/10826-go-metrics/)。有部分PromQL函数和表达式未支持，所以有个别图没有数据。
![screencapture-go-metrics-2023-01-12-16_37_22.png](./screencapture-go-metrics-2023-01-12-16_37_22.png)

## 应用场景
![promql2influxql.png](./promql2influxql.png)

如果你想用InfluxDB作为时序数据的底层存储，同时又希望能继续使用Prometheus的PromQL查询语句做数据分析，可以采用promql2influxql替换掉Prometheus的接口服务，仅将Prometheus用作监控数据采集服务。

## UML类图
![uml.png](./uml.png)

## Prometheus数据写入InfluxDB格式转换
```shell
# Prometheus metric
example_metric{queue="0:http://example:8086/api/v1/prom/write?db=prometheus",le="0.005"} 308

# Same metric parsed into InfluxDB
measurement
  example_metric
tags
  queue = "0:http://example:8086/api/v1/prom/write?db=prometheus"
  le = "0.005"
  job = "prometheus"
  instance = "localhost:9090"
  __name__ = "example_metric"
fields
  value = 308
```

## 查询结果数据格式
```json
{
  "resultType": "vector",
  "result": [
    {
      "metric": {
        "container": "alertmanager",
        "endpoint": "web",
        "instance": "172.17.0.4:9093",
        "job": "alertmanager-main",
        "namespace": "monitoring",
        "pod": "alertmanager-main-0",
        "service": "alertmanager-main"
      },
      "value": [
        1672995857.892,
        "8060"
      ]
    }
  ]
}
```

## 使用方式
本项目有两种使用方式：第三方库、RESTful服务等
### 第三方库
直接在你的项目根路径下执行`go get`命令即可。
```shell
go get -d github.com/wubin1989/promql2influxql@v0.0.1
```

### RESTful服务
RESTful服务代码在`promql/rpc`路径下，是一个单独的go模块。已经有了Dockerfile和docker-compose.yml文件。推荐测试环境采用docker方式部署。

#### 架构设计
![architecture.png](./architecture.png)

#### 本地启动
```shell
go run cmd/main.go
```
可看到如下命令行日志输出：
```shell
➜  rpc git:(main) ✗ go run cmd/main.go                                 
2023/01/12 19:57:18 maxprocs: Leaving GOMAXPROCS=16: CPU quota undefined
                           _                    _
                          | |                  | |
  __ _   ___   ______   __| |  ___   _   _   __| |  ___   _   _
 / _` | / _ \ |______| / _` | / _ \ | | | | / _` | / _ \ | | | |
| (_| || (_) |        | (_| || (_) || |_| || (_| || (_) || |_| |
 \__, | \___/          \__,_| \___/  \__,_| \__,_| \___/  \__,_|
  __/ |
 |___/
2023-01-12 19:57:18 INF ================ Registered Routes ================
2023-01-12 19:57:18 INF +---------------------------+--------+----------------------------------+
2023-01-12 19:57:18 INF |           NAME            | METHOD |             PATTERN              |
2023-01-12 19:57:18 INF +---------------------------+--------+----------------------------------+
2023-01-12 19:57:18 INF | Query                     | POST   | /api/v1/query                    |
2023-01-12 19:57:18 INF | GetQuery                  | GET    | /api/v1/query                    |
2023-01-12 19:57:18 INF | Query_range               | POST   | /api/v1/query_range              |
2023-01-12 19:57:18 INF | GetQuery_range            | GET    | /api/v1/query_range              |
2023-01-12 19:57:18 INF | GetLabel_Label_nameValues | GET    | /api/v1/label/:label_name/values |
2023-01-12 19:57:18 INF | GetDoc                    | GET    | /go-doudou/doc                   |
2023-01-12 19:57:18 INF | GetOpenAPI                | GET    | /go-doudou/openapi.json          |
2023-01-12 19:57:18 INF | Prometheus                | GET    | /go-doudou/prometheus            |
2023-01-12 19:57:18 INF | GetConfig                 | GET    | /go-doudou/config                |
2023-01-12 19:57:18 INF | GetStatsvizWs             | GET    | /go-doudou/statsviz/ws           |
2023-01-12 19:57:18 INF | GetStatsviz               | GET    | /go-doudou/statsviz/*            |
2023-01-12 19:57:18 INF | GetDebugPprofCmdline      | GET    | /debug/pprof/cmdline             |
2023-01-12 19:57:18 INF | GetDebugPprofProfile      | GET    | /debug/pprof/profile             |
2023-01-12 19:57:18 INF | GetDebugPprofSymbol       | GET    | /debug/pprof/symbol              |
2023-01-12 19:57:18 INF | GetDebugPprofTrace        | GET    | /debug/pprof/trace               |
2023-01-12 19:57:18 INF | GetDebugPprofIndex        | GET    | /debug/pprof/*                   |
2023-01-12 19:57:18 INF +---------------------------+--------+----------------------------------+
2023-01-12 19:57:18 INF ===================================================
2023-01-12 19:57:18 INF Http server is listening at :9090
2023-01-12 19:57:18 INF Http server started in 6.225365ms
```

在线Swagger接口文档地址：http://localhost:9090/go-doudou/doc   
接口文档http basic用户名/密码：admin/admin

#### 测试环境
打包docker镜像
```shell
docker build -t promql2influxql_promql2influxql .
```

启动RESTful服务和基础设施容器
```shell
docker-compose -f docker-compose.yml up -d --remove-orphans
```
可以看到如下命令行日志输出
```shell
➜  rpc git:(main) ✗ docker-compose -f docker-compose.yml up -d --remove-orphans
[+] Running 6/6
 ⠿ Network rpc_default                        Created                                                                                                                                  0.1s
 ⠿ Container promql2influxql_influxdb         Started                                                                                                                                  1.1s
 ⠿ Container promql2influxql_node_exporter    Started                                                                                                                                  0.3s
 ⠿ Container promql2influxql_promql2influxql  Started                                                                                                                                  1.0s
 ⠿ Container promql2influxql_grafana          Started                                                                                                                                  1.0s
 ⠿ Container promql2influxql_prometheus       Started 
```
以下是各服务的请求地址：
- promql2influxql服务：`http://promql2influxql_promql2influxql:9090`（需要配置到grafana数据源）
- promql2influxql服务在线Swagger接口文档地址：http://localhost:9091/go-doudou/doc  
  接口文档http basic用户名/密码：admin/admin
- Grafana：`http://localhost:3000`
- Prometheus：`http://localhost:9090`（仅用作监控数据采集服务）
- Influxdb：`http://promql2influxql_influxdb:8086`

## TODO
### 指标类型
- [x] Counter：计数器
- [x] Gauge：仪表盘
- [x] Histogram：直方图
- [x] Summary：摘要
### 选择器（8个）
- [x] =：相等匹配器
- [x] !=：不相等匹配器
- [x] =~：正则表达式匹配器
- [x] !~：正则表达式相反匹配器
- [x] {}：瞬时向量选择器
- [x] {}[]：区间向量选择器  
  ~~- [ ] {}\[:\]：子查询~~（原生influxql不支持）
- [x] offset：偏移量修改器
### 聚合操作（13个）
- [x] by：相当于InfluxQL的group by语句  
  ~~- [ ] without：忽略指定标签，by的相反操作~~（原生influxql不支持）
- [x] sum：求和
- [x] min：最小值
- [x] max：最大值
- [x] avg：平均值
- [x] stddev：标准差  
  ~~- [ ] stdvar：标准差异~~（原生influxql不支持）
- [x] count：统计结果行数  
  ~~- [ ] count_values：按值分组，统计每组的结果行数~~（原生influxql不支持）
- [x] bottomk：样本值最小的k个元素
- [x] topk：样本值最大的k个元素  
- [x] quantile：分布统计
### 二元操作符（20个）
- [x] +：加法
- [x] -：减法
- [x] x：乘法
- [x] /：除法
- [x] %：取模
- [x] ^：求幂
- [x] and：且
- [x] or：或    
  ~~- [ ] unless：排除~~（原生influxql不支持）   
  ~~- [ ] ==：等于~~（原生influxql不支持）   
- [x] !=：不等于
- [x] \>：大于
- [x] <：小于
- [x] \>=：大于等于
- [x] <=：小于等于  
  ~~- [ ] bool：0表示false，1表示true~~（原生influxql不支持）  
  ~~- [ ] ignoring：忽略标签~~（原生influxql不支持）  
  ~~- [ ] on：与ignoring相反，类似by~~（原生influxql不支持）  
  ~~- [ ] group_left：多对一，类似sql的左连接~~（原生influxql不支持）  
  ~~- [ ] group_right：一对多，类似sql的右连接~~（原生influxql不支持）  
### 内置函数（共70个，已支持24个）
根据官方文档 [https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions](https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions) 整理
- [x] abs()  
  ~~- [ ] absent()~~（原生influxql不支持）
  ~~- [ ] absent_over_time()~~（原生influxql不支持）
- [x] ceil()  
  ~~- [ ] changes()~~（原生influxql不支持）    
- [ ] clamp()：按最大值、最小值区间范围筛选  
- [ ] clamp_max()：按最大值筛选  
- [ ] clamp_min()：按最小值筛选    
  ~~- [ ] day_of_month()~~（原生influxql不支持）    
  ~~- [ ] day_of_week()~~（原生influxql不支持）    
  ~~- [ ] day_of_year()~~（原生influxql不支持）    
  ~~- [ ] days_in_month()~~（原生influxql不支持）  
  ~~- [ ] delta()~~（原生influxql不支持）  
- [x] deriv()
- [x] exp()
- [x] floor()  
  ~~- [ ] histogram_count()~~（原生influxql不支持）  
  ~~- [ ] histogram_sum()~~（原生influxql不支持）  
  ~~- [ ] histogram_fraction()~~（原生influxql不支持）  
  ~~- [ ] histogram_quantile()~~（原生influxql不支持）  
- [ ] holt_winters()    
  ~~- [ ] hour()~~（原生influxql不支持）    
- [ ] idelta()
- [ ] increase()
- [ ] irate()
- [ ] label_join()
- [ ] label_replace()
- [x] ln()
- [x] log2()
- [x] log10()    
  ~~- [ ] minute()~~（原生influxql不支持）    
  ~~- [ ] month()~~（原生influxql不支持）    
- [ ] predict_linear()
- [x] rate()
- [ ] resets()
- [x] round()
- [ ] scalar()
- [ ] sgn()      
  ~~- [ ] sort()~~：InfluxDB只支持order by time，Prometheus只支持order by value      
  ~~- [ ] sort_desc()~~：InfluxDB只支持order by time，Prometheus只支持order by value    
- [x] sqrt()
- [ ] time()
- [ ] timestamp()
- [ ] vector()    
  ~~- [ ] year()~~（原生influxql不支持）    
- [x] avg_over_time()
- [x] min_over_time()
- [x] max_over_time()
- [x] sum_over_time()
- [x] count_over_time()
- [x] quantile_over_time()
- [x] stddev_over_time()    
  ~~- [ ] stdvar_over_time()~~（原生influxql不支持）  
- [ ] last_over_time()  
  ~~- [ ] present_over_time()~~（原生influxql不支持）  
- [x] acos()      
  ~~- [ ] acosh()~~（原生influxql不支持）    
- [x] asin()    
  ~~- [ ] asinh()~~（原生influxql不支持）    
- [x] atan()    
  ~~- [ ] atanh()~~（原生influxql不支持）    
- [x] cos()    
  ~~- [ ] cosh()~~（原生influxql不支持）    
- [x] sin()    
  ~~- [ ] sinh()~~（原生influxql不支持）    
- [x] tan()    
  ~~- [ ] tanh()~~（原生influxql不支持）    
  ~~- [ ] deg()~~（原生influxql不支持）    
  ~~- [ ] pi()~~（原生influxql不支持）    
  ~~- [ ] rad()~~（原生influxql不支持）   

## 其他说明

### 关于查询时间范围
- 结束时间取值优先级从最高到最低依次是：
  - `End`参数
  - PromQL查询命令中的`@`表达式
  - `Evaluation`参数
  - 当前时间
  以上的结果会跟PromQL查询命令中的`offset`表达式再计算得出最终的结束时间
- 开始时间只取`Start`参数

### 关于图表数据查询
因为原生InfluxQL不支持Prometheus的`/api/v1/query_range`接口的`step`参数和相应的计算机制，例如一段时间范围内，每隔3分钟，计算一次前10分钟的http请求增长速率，原生InfluxQL只能做到利用`group by time(3m)`语句实现一段时间范围内每隔3分钟，计算一次前3分钟的http请求增长速率，所以本项目对此的处理方式是：当PromQL查询语句中包含区间向量查询，例如`go_gc_duration_seconds_count[5m]`中的`[5m]`，同时传了`Step`参数，则忽略`Step`参数，取区间时间范围的`5m`作为`group by time(interval)`语句中的`interval`参数值。

### 暂不支持PromQL多measurement查询和二元操作符两边同时为VectorSelector或MatrixSelector表达式查询
原生InfluxQL语句实现不了，后续计划通过进行多次InfluxQL查询后在内存中计算实现。

## Credits
本项目参考了 [https://github.com/influxdata/flux](https://github.com/influxdata/flux) 项目的PromQL转Flux转译器的代码。此外，还依赖了很多非常优秀的开源项目。在此向各位开源作者表示感谢！

## License
MIT
