package executors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/promql"
	"github.com/prometheus/prometheus/rules"
	"gopkg.in/yaml.v2"
)

func toModelSample(s promql.Sample) *model.Sample {
	ls := make(model.Metric)
	for _, l := range s.Metric {
		ls[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	}

	return &model.Sample{
		Value:     model.SampleValue(s.Point.V),
		Timestamp: model.Time(s.Point.T),
		Metric:    ls,
	}
}

type series struct {
	Series string `yaml:"series"`
	Values string `yaml:"values"`
}

//TODO: The name of this should be changed
type testSeries struct {
	InputSeries []series `yaml:"input_series"`
	Interval    string   `yaml:"interval"`
}

func (t *testSeries) seriesLoadingString() string {

	//TODO: check why we need shortDuration
	result := fmt.Sprintf("load %v\n", t.Interval)
	for _, is := range t.InputSeries {
		result += fmt.Sprintf("  %v %v\n", is.Series, is.Values)
	}
	return result
}

func loadFromFile(filename string) (*testSeries, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	res := new(testSeries)
	err = yaml.Unmarshal(b, res)

	if err != nil {
		return nil, err
	}
	return res, nil
}

type PromqlExecutor struct {
	ll       promql.LazyLoader
	interval time.Duration
}

// NewPromqlExecutor creaRtes new executor with time series and rules loaded from file
// Todo: Should somehow close ll after we are done
func NewPromqlExecutor(timeSeriesFile, ruleFile string) *PromqlExecutor {
	f, err := loadFromFile(timeSeriesFile)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	interval, err := time.ParseDuration(f.Interval)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	//Load time series from file
	ll, err := promql.NewLazyLoader(nil, f.seriesLoadingString())
	if err != nil {
		fmt.Println(err)
		return nil
	}

	//Load rule groups
	opts := &rules.ManagerOptions{
		QueryFunc:  rules.EngineQueryFunc(ll.QueryEngine(), ll.Storage()),
		Appendable: ll.Storage(),
		Context:    context.Background(),
		NotifyFunc: func(ctx context.Context, expr string, alerts ...*rules.Alert) {},
		Logger:     nil,
	}
	m := rules.NewManager(opts)
	groupsMap, ers := m.LoadGroups(interval, nil, ruleFile)
	if ers != nil {
		fmt.Println(ers)
		return nil
	}

	//Load data into ll
	ll.WithSamplesTill(time.Now(), func(e error) {
		if err != nil {
			err = e
		}
	})
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// Evaluate rules after data was loaded
	// Assuming, no one will insert test data with duration longer than 60 seconds
	// TODO: extract the length of longest sequence and use it for calculation of max time
	maxt := time.Unix(0, 0).UTC().Add(time.Duration(60) * time.Minute)
	for ts := time.Unix(0, 0).UTC(); ts.Before(maxt) || ts.Equal(maxt); ts = ts.Add(interval) {
		for _, g := range groupsMap {
			g.Eval(context.Background(), ts)
		}
	}

	return &PromqlExecutor{ll: *ll, interval: interval}
}

// Query queries prometheus mock engine with data loaded from file
// The start date for all queries is time.Time.UTC(0,0)
func (p *PromqlExecutor) Query(query string, queryTime time.Time) ([]*model.Sample, error) {
	qe := p.ll.QueryEngine()
	q, err := qe.NewInstantQuery(p.ll.Queryable(), query, queryTime)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer q.Close()
	res := q.Exec(p.ll.Context())

	switch v := res.Value.(type) {
	case promql.Vector:
		res := make([]*model.Sample, 0, len(v))
		for _, s := range v {
			res = append(res, toModelSample(s))
		}
		return res, nil
	case promql.Scalar:
		return nil, errors.New("rule result is not a vector")
	default:
		return nil, errors.New("rule result is not a vector")
	}
}

// func main() {
// 	p := NewPromqlExecutor("test.yml", "rules.yml")
// 	p.Query("sum(rate(up{job=\"prometheus\", instance=\"localhost:9090\"}[5m]))", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))
// 	p.Query("prometheus", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))

// 	// p.Query("sum(rate(apiserver_request_duration_seconds_bucket[1m]))", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))
// 	p.Query("my_own_sum", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))

// 	p.Query("quantile_over_time(0.99, apiserver:apiserver_request_latency_1m:histogram_quantile{verb!=\"WATCH\", subresource!~\"log|exec|portforward|attach|proxy\"}[2m])", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))

// 	// p.Query("quantile_over_time(0.99, apiserver:apiserver_request_latency_1m:histogram_quantile{verb!=\"WATCH\", subresource!~\"log|exec|portforward|attach|proxy\"}[1m])", time.Unix(0, 0).UTC().Add(time.Duration(5)*time.Minute))

// 	//query "quantile_over_time(0.99, apiserver:apiserver_request_latency_1m:histogram_quantile{verb!=\"WATCH\", subresource!~\"log|exec|portforward|attach|proxy\"}[1m])"
// 	//count query "sum(increase(apiserver_request_duration_seconds_count{verb!=\"WATCH\", subresource!~\"log|exec|portforward|attach|proxy\"}[43s])) by (resource, subresource, scope, verb)"

// 	//count fast query "sum(increase(apiserver_request_duration_seconds_bucket{verb!~\"WATCH|LIST\", subresource!=\"proxy\", le=\"1\"}[43s])) by (resource, subresource, scope, verb)"
// 	//"sum(increase(apiserver_request_duration_seconds_bucket{scope!=\"cluster\", verb=\"LIST\", le=\"5\"}[43s])) by (resource, subresource, scope, verb)"
// 	//"sum(increase(apiserver_request_duration_seconds_bucket{scope=\"cluster\", verb=\"LIST\", le=\"30\"}[43s])) by (resource, subresource, scope, verb)"

// }
