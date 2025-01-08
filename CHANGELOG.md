## 0.11.0 / 2025-01-08

* [ENHANCEMENT] Update Go to 1.23.x and update all Go module dependencies (#27)

## 0.10.0 / 2023-05-23

* [ENHANCEMENT] Update to Go 1.20 and update Go module dependencies (#26)

## 0.9.0 / 2021-11-04

* [ENHANCEMENT] Use Go 1.17
* [ENHANCEMENT] Update Go module dependencies

## 0.8.1 / 2021-11-03

* [BUGFIX] Fix parsing of no instances with latest Passenger

## 0.8.0 / 2020-11-25

* [ENHANCEMENT] Allow flags to be defined using environment variables

## 0.7.0 / 2020-11-17

* [ENHANCEMENT] Make source install easier

## 0.6.2 / 2020-11-17

* [BUGFIX] Fix Passenger collector to PUN UIDs when falling back to PID lookup

## 0.6.1 / 2020-11-17

* [BUGFIX] Fix issue where 1 PUN would result in no passenger metrics

## 0.6.0 / 2020-10-28

* Use Prometheus procfs module for process collection
* Replace ondemand_pun_cpu_percent with ondemand_pun_cpu_time
* Update to Go 1.15 and update dependencies

## 0.5.2 / 2020-06-04

* Minor documentation fixes #13

## 0.5.1 / 2020-05-12

* Fix path for ondemand-passenger-status

## 0.5.0 / 2020-04-28

* Update to Go 1.14 and update Prometheus client dependency

## 0.4.0 / 2020-04-27

* Add passenger metrics
* Rename --listen flag to --web.listen-address
* Rename --apache-status  to --collector.apache.status-url
* Fix pun process CPU metric

## 0.3.0 / 2020-03-05

### Changes

* Use promlog for loggin

## 0.2.0 / 2020-03-02

### Changes

* Support timeouts for collectors
* Improve metrics for collector errors

## 0.1.0 / 2020-02-25

### Changes

* Update client_golang dependency
* Improved testing

## 0.0.2 / 2020-02-15

### Changes

* Use Makefiles from Prometheus exporters
* Build for more architectures

## 0.0.1

### Changes

* Initial Release

