//go:build quosparr_dev_package

package app

// CI packages use mygo's production renderer but intentionally omit updates.
const developmentPackage = true
