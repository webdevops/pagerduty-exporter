package main

import (
	"github.com/mblaschke/go-pagerduty"
	"github.com/prometheus/client_golang/prometheus"
)

func collectUser(callback chan<- func()) {
	listOpts := pagerduty.ListUsersOptions{}
	listOpts.Limit = PAGERDUTY_LIST_LIMIT
	listOpts.Offset = 0

	userList := []prometheusEntry{}

	for {
		Logger.Verbosef(" - fetch users (offset: %v, limit:%v)", listOpts.Offset, listOpts.Limit)

		list, err := PagerDutyClient.ListUsers(listOpts)
		prometheusApiCounter.WithLabelValues("ListUsers").Inc()

		if err != nil {
			panic(err)
		}

		for _, user := range list.Users {
			row := prometheusEntry{
				labels: prometheus.Labels{
					"userID": user.ID,
					"userName": user.Name,
					"userMail": user.Email,
					"userAvatar": user.AvatarURL,
					"userColor": user.Color,
					"userJobTitle": user.JobTitle,
					"userRole": user.Role,
					"userTimezone": user.Timezone,
				},
				value: 1,
			}
			userList = append(userList, row)
		}

		listOpts.Offset += list.Limit
		if !list.More {
			break
		}
	}

	// set metrics
	callback <- func() {
		for _, row := range userList {
			prometheusUser.With(row.labels).Set(row.value)
		}
	}
}

