// Package estimate ports QuotaEstimate.swift, QuotaEstimateAvailability.swift
// and QuotaEstimateHistoryStore.swift: how many Credits a full quota window is
// worth, extrapolated from the Credits used so far in the current cycle.
//
// The calculation needs the quota windows and the daily analytics rows. Those
// models belong to other packages, so Window, Quota and DailyRow here are the
// minimal inputs; convert from the real types at the call site.
//
// History is kept per account in ~/.quosparr/quota-estimate-history.json,
// byte-compatible with the Swift app so an upgraded user keeps the history.
package estimate
