// Package prometheus provides Prometheus support for ecobee metrics.
package collector

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/billykwooten/go-ecobee/ecobee"
	"github.com/prometheus/client_golang/prometheus"
)

type descs string

func (d descs) new(fqName, help string, variableLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(fmt.Sprintf("%s_%s", d, fqName), help, variableLabels, nil)
}

// eCollector implements prometheus.eCollector to gather ecobee metrics on-demand.
type eCollector struct {
	client *ecobee.Client

	// per-query descriptors
	fetchTime *prometheus.Desc

	// runtime descriptors
	actualTemperature, targetTemperatureMin, targetTemperatureMax *prometheus.Desc

	// sensor descriptors
	temperature, humidity, occupancy, inUse, currentHvacMode, voc, co2, air_quality, air_quality_accuracy, air_pressure *prometheus.Desc

	// equipment
	equipmentRunning, auxHeat1, auxHeat2, auxHeat3, compCool1, compCool2, heatPump1, heatPump2, fan *prometheus.Desc

	// oat
	outsideTempF, outsideTemp *prometheus.Desc
}

// NewEcobeeCollector returns a new eCollector with the given prefix assigned to all
// metrics. Note that Prometheus metrics must be unique! Don't try to create
// two Collectors with the same metric prefix.
func NewEcobeeCollector(c *ecobee.Client, metricPrefix string) *eCollector {
	d := descs(metricPrefix)

	// fields common across multiple metrics
	runtime := []string{"thermostat_id", "thermostat_name"}
	sensor := append(runtime, "sensor_id", "sensor_name", "sensor_type")

	return &eCollector{
		client: c,

		// collector metrics
		fetchTime: d.new(
			"fetch_time",
			"elapsed time fetching data via Ecobee API",
			nil,
		),

		// thermostat (aka runtime) metrics
		actualTemperature: d.new(
			"actual_temperature",
			"thermostat-averaged current temperature",
			runtime,
		),
		targetTemperatureMax: d.new(
			"target_temperature_max",
			"maximum temperature for thermostat to maintain",
			runtime,
		),
		targetTemperatureMin: d.new(
			"target_temperature_min",
			"minimum temperature for thermostat to maintain",
			runtime,
		),

		// sensor metrics
		temperature: d.new(
			"temperature",
			"temperature reported by a sensor in degrees",
			sensor,
		),
		humidity: d.new(
			"humidity",
			"humidity reported by a sensor in percent",
			sensor,
		),
		occupancy: d.new(
			"occupancy",
			"occupancy reported by a sensor (0 or 1)",
			sensor,
		),
		voc: d.new(
			"volitile_organic_compounds_ppm",
			"VOCs",
			sensor,
		),
		co2: d.new(
			"carbon_dioxide_ppm",
			"CO2",
			sensor,
		),
		air_quality: d.new(
			"air_quality",
			"air quality",
			sensor,
		),
		air_quality_accuracy: d.new(
			"air_quality_accuracy",
			"air quality accuracy",
			sensor,
		),
		air_pressure: d.new(
			"air_pressure",
			"air pressure in ??",
			sensor,
		),
		inUse: d.new(
			"in_use",
			"is sensor being used in thermostat calculations (0 or 1)",
			sensor,
		),
		currentHvacMode: d.new(
			"currenthvacmode",
			"current hvac mode of thermostat",
			[]string{"thermostat_id", "thermostat_name", "current_hvac_mode"},
		),
		outsideTemp: d.new(
			"outside_temperature",
			"current outside temperature (Celsius)",
			runtime,
		),
		outsideTempF: d.new(
			"outside_temperature_fahrenheit",
			"current outside temperature (Fahrenheit)",
			runtime,
		),
		equipmentRunning: d.new(
			"equipment_running",
			"equipment currently running (1 for on, 0 for off)",
			append(runtime, "name"),
		),
		auxHeat1: d.new(
			"aux_heat1",
			"Heat stage 1",
			runtime,
		),
		auxHeat2: d.new(
			"aux_heat2",
			"Heat stage 2",
			runtime,
		),
		auxHeat3: d.new(
			"aux_heat3",
			"Heat stage 3",
			runtime,
		),
		compCool1: d.new(
			"comp_cool1",
			"Cool stage 1",
			runtime,
		),
		compCool2: d.new(
			"comp_cool2",
			"Cool stage 2",
			runtime,
		),
		heatPump1: d.new(
			"heat_pump1",
			"Heat pump stage 1",
			runtime,
		),
		heatPump2: d.new(
			"heat_pump2",
			"Heat pump stage 2",
			runtime,
		),
		fan: d.new(
			"fan",
			"current hvac mode of thermostat",
			runtime,
		),
	}
}

// Describe dumps all metric descriptors into ch.
func (c *eCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.fetchTime
	ch <- c.actualTemperature
	ch <- c.targetTemperatureMax
	ch <- c.targetTemperatureMin
	ch <- c.temperature
	ch <- c.humidity
	ch <- c.voc
	ch <- c.co2
	ch <- c.air_quality
	ch <- c.air_quality_accuracy
	ch <- c.air_pressure
	ch <- c.occupancy
	ch <- c.inUse
	ch <- c.currentHvacMode
	ch <- c.outsideTemp
	ch <- c.outsideTempF
	ch <- c.equipmentRunning
	ch <- c.auxHeat1
	ch <- c.auxHeat2
	ch <- c.auxHeat3
	ch <- c.compCool1
	ch <- c.compCool2
	ch <- c.heatPump1
	ch <- c.heatPump2
	ch <- c.fan
}

func is_thing_running(values []int) float64 {
	var r = 0.0
	for _, v := range values {
		if v > 0 {
			r = 1.0
		}

	}
	return r
}

func ecobee_temp_in_f(value int) float64 {
	return float64(value)/10
}

func ecobee_temp_in_c(value int) float64 {
	return (float64(value)/10 - 32)*5/9
}

// Collect retrieves thermostat data via the ecobee API.
func (c *eCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	tt, err := c.client.GetThermostats(ecobee.Selection{
		SelectionType:   "registered",
		IncludeSensors:  true,
		IncludeRuntime:  true,
		IncludeExtendedRuntime:  true,
		IncludeSettings: true,
		IncludeWeather: true,
	})
	elapsed := time.Now().Sub(start)
	ch <- prometheus.MustNewConstMetric(c.fetchTime, prometheus.GaugeValue, elapsed.Seconds())
	if err != nil {
		log.Error(err)
		return
	}
	// https://developer.ecobee.com/home/developer/api/documentation/v1/objects/Thermostat.shtml
	// https://developer.ecobee.com/home/developer/api/documentation/v1/objects/Weather.shtml
	for _, t := range tt {
		tFields := []string{t.Identifier, t.Name}
		if t.Runtime.Connected {
			for _, w := range t.Weather.Forecasts {
				log.Infof("At %q, temp is %fC", w.DateTime, ecobee_temp_in_c(w.Temperature))
			}

			ch <- prometheus.MustNewConstMetric(
				c.actualTemperature, prometheus.GaugeValue, ecobee_temp_in_f(t.Runtime.ActualTemperature), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.targetTemperatureMax, prometheus.GaugeValue, ecobee_temp_in_f(t.Runtime.DesiredCool), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.targetTemperatureMin, prometheus.GaugeValue, ecobee_temp_in_f(t.Runtime.DesiredHeat), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.currentHvacMode, prometheus.GaugeValue, 0, t.Identifier, t.Name, t.Settings.HvacMode,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat1), append(tFields, "heat 1")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat2), append(tFields, "heat 2")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat3), append(tFields, "heat 3")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Cool1), append(tFields, "cooling 1")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Cool2), append(tFields, "cooling 2")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.HeatPump1), append(tFields, "heat pump 1")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.HeatPump2), append(tFields, "heat pump 2")...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.equipmentRunning, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Fan), append(tFields, "fan")...,
			)

			ch <- prometheus.MustNewConstMetric(
				c.outsideTempF, prometheus.GaugeValue, ecobee_temp_in_f(t.Weather.Forecasts[0].Temperature), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.outsideTemp, prometheus.GaugeValue, ecobee_temp_in_c(t.Weather.Forecasts[0].Temperature), tFields...,
			)


			ch <- prometheus.MustNewConstMetric(
				c.auxHeat1, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat1), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.auxHeat2, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat2), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.auxHeat3, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.AuxHeat3), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.compCool1, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Cool1), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.compCool2, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Cool2), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.heatPump1, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.HeatPump1), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.heatPump2, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.HeatPump2), tFields...,
			)
			ch <- prometheus.MustNewConstMetric(
				c.fan, prometheus.GaugeValue, is_thing_running(t.ExtendedRuntime.Fan), tFields...,
			)
		}
		for _, s := range t.RemoteSensors {
			sFields := append(tFields, s.ID, s.Name, s.Type)
			inUse := float64(0)
			if s.InUse {
				inUse = 1
			}
			ch <- prometheus.MustNewConstMetric(
				c.inUse, prometheus.GaugeValue, inUse, sFields...,
			)
			for _, sc := range s.Capability {
				switch sc.Type {
				case "temperature":
					if v, err := strconv.ParseInt(sc.Value, 10, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.temperature, prometheus.GaugeValue, ecobee_temp_in_f(int(v)), sFields...,
						)
					} else {
						log.Error(err)
					}
				case "humidity":
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.humidity, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Error(err)
					}
				case "occupancy":
					switch sc.Value {
					case "true":
						ch <- prometheus.MustNewConstMetric(
							c.occupancy, prometheus.GaugeValue, 1, sFields...,
						)
					case "false":
						ch <- prometheus.MustNewConstMetric(
							c.occupancy, prometheus.GaugeValue, 0, sFields...,
						)
					default:
						log.Errorf("unknown sensor occupancy value %q", sc.Value)
					}
				case "vocPPM":
					if sc.Value == "unknown" {
						continue
					}
					
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.voc, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Errorf("value [%q] was %q", sc.Type, sc.Value)
						log.Error(err)
					}
				case "co2PPM":
					if sc.Value == "unknown" {
						continue
					}
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.co2, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Errorf("value [%q] was %q", sc.Type, sc.Value)
						log.Error(err)
					}
				case "airQualityAccuracy":
					if sc.Value == "unknown" {
						continue
					}
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.air_quality_accuracy, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Errorf("value [%q] was %q", sc.Type, sc.Value)
						log.Error(err)
					}
				case "airQuality":
					if sc.Value == "unknown" {
						continue
					}
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.air_quality, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Errorf("value [%q] was %q", sc.Type, sc.Value)
						log.Error(err)
					}
				case "airPressure":
					if sc.Value == "unknown" {
						continue
					}
					if v, err := strconv.ParseFloat(sc.Value, 64); err == nil {
						ch <- prometheus.MustNewConstMetric(
							c.air_pressure, prometheus.GaugeValue, v, sFields...,
						)
					} else {
						log.Errorf("value [%q] was %q", sc.Type, sc.Value)
						log.Error(err)
					}
				default:
					log.Infof("ignoring sensor capability %q", sc.Type)
				}
			}
		}
	}
}
