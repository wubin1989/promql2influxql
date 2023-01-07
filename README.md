# promql2influxql
本项目是PromQL转InfluxQL转译器和适配器，实现了传入原生PromQL查询语句，转成InfluxQL语句，并查询InfluxDB数据库返回结果。项目还在快速迭代中，请勿用于生产环境。

## 项目说明
本程序基于以下前置条件开发：
- 基础设施版本：
  - Prometheus v2.41.0：Docker镜像 [prom/prometheus:v2.41.0](https://hub.docker.com/r/prom/prometheus)
  - InfluxDB v1.8.10：Docker镜像 [influxdb:1.8.10](https://hub.docker.com/_/influxdb)
- Prometheus数据写入方式：
  - [Prometheus Remote Write机制](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write)
  - [InfluxDB的/api/v1/prom/write接口](https://docs.influxdata.com/influxdb/v1.8/supported_protocols/prometheus/)
- 查询结果数据格式：参考Prometheus的`/api/v1/query_range`接口

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
~~- [ ] without：忽略标签~~（原生influxql不支持）
- [x] by：与without相反
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
~~- [ ] quantile：分布统计~~（原生influxql不支持）
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
### 内置函数（70个）
根据官方文档 [https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions](https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions) 整理
- [ ] abs()
- [ ] absent()
- [ ] absent_over_time()
- [ ] ceil()
- [ ] changes()
- [ ] clamp()
- [ ] clamp_max()
- [ ] clamp_min()
- [ ] day_of_month()
- [ ] day_of_week()
- [ ] day_of_year()
- [ ] days_in_month()
- [ ] delta()
- [ ] deriv()
- [ ] exp()
- [ ] floor()
- [ ] histogram_count()
- [ ] histogram_sum()
- [ ] histogram_fraction()
- [ ] histogram_quantile()
- [ ] holt_winters()
- [ ] hour()
- [ ] idelta()
- [ ] increase()
- [ ] irate()
- [ ] label_join()
- [ ] label_replace()
- [ ] ln()
- [ ] log2()
- [ ] log10()
- [ ] minute()
- [ ] month()
- [ ] predict_linear()
- [ ] rate()
- [ ] resets()
- [ ] round()
- [ ] scalar()
- [ ] sgn()  
~~- [ ] sort()~~：InfluxDB只支持order by time，Prometheus只支持order by value  
~~- [ ] sort_desc()~~：InfluxDB只支持order by time，Prometheus只支持order by value
- [ ] sqrt()
- [ ] time()
- [ ] timestamp()
- [ ] vector()
- [ ] year()
- [ ] avg_over_time()
- [ ] min_over_time()
- [ ] max_over_time()
- [ ] sum_over_time()
- [ ] count_over_time()
- [ ] quantile_over_time()
- [ ] stddev_over_time()
- [ ] stdvar_over_time()
- [ ] last_over_time()
- [ ] present_over_time()
- [ ] acos()
- [ ] acosh()
- [ ] asin()
- [ ] asinh()
- [ ] atan()
- [ ] atanh()
- [ ] cos()
- [ ] cosh()
- [ ] sin()
- [ ] sinh()
- [ ] tan()
- [ ] tanh()
- [ ] deg()
- [ ] pi()
- [ ] rad()