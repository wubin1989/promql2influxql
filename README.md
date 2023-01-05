# promql2influxql

## TODO
### 选择器（8个）
- [ ] =：相等匹配器
- [ ] !=：不相等匹配器
- [ ] =~：正则表达式匹配器
- [ ] !~：正则表达式相反匹配器
- [ ] {}：瞬时向量选择器
- [ ] {}[]：区间向量选择器  
~~- [ ] {}\[:\]：子查询~~（influxql不支持）
- [ ] offset：偏移量修改器
### 聚合操作（13个）
- [ ] without：忽略标签
- [ ] by：与without相反
- [ ] sum：求和
- [ ] min：最小值
- [ ] max：最大值
- [ ] avg：平均值
- [ ] stddev：标准差
- [ ] stdvar：标准差异
- [ ] count：计数
- [ ] count_values：对value计数
- [ ] bottomk：样本值最小的k个元素
- [ ] topk：样本值最大的k个元素
- [ ] quantile：分布统计
### 二元操作符（20个）
- [ ] +：加法
- [ ] -：减法
- [ ] x：乘法
- [ ] /：除法
- [ ] %：取模
- [ ] ^：求幂
- [ ] and：且
- [ ] or：或
- [ ] unless：排除
- [ ] ==：等于
- [ ] !=：不等于
- [ ] \>：大于
- [ ] <：小于
- [ ] \>=：大于等于
- [ ] <=：小于等于
- [ ] bool：0表示false，1表示true
- [ ] ignoring：忽略标签
- [ ] on：与ignoring相反，类似by
- [ ] group_left：多对一，类似sql的左连接
- [ ] group_right：一对多，类似sql的右连接
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
- [ ] sort()
- [ ] sort_desc()
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