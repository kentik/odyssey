package controllers

import (
	"context"
	"fmt"

	syntheticsv1 "github.com/kentik/odyssey/api/v1"
	"github.com/pkg/errors"
)

func (r *SyntheticTaskReconciler) updateHandler() {
	for {
		updateConfig := <-r.updateCh
		task := updateConfig.syntheticTask
		log := r.Log.WithValues("name", task.Name, "namespace", task.Namespace)
		configData := ""

		log.Info("handling update", "task", task)
		// TODO: build new server configuration and reload pods
		for _, i := range updateConfig.tasks {
			switch v := i.(type) {
			case *syntheticsv1.Fetch:
				log.Info("configuring fetch task", "fetch", fmt.Sprintf("%+v", v))
				yml, err := v.Yaml()
				if err != nil {
					log.Error(err, fmt.Sprintf("%+T", i))
				}
				configData += yml
			default:
				log.Error(errors.New("unknown type for task"), fmt.Sprintf("%+T", i))
				continue
			}
		}

		log.Info("updating server config", "task", task.Name)
		// TODO: update service config
		configMap := updateConfig.serverConfigMap
		configMap.Data[serverConfigMapName] = configData
		if err := r.Update(context.Background(), configMap); err != nil {
			log.Error(err, task.Name)
			continue
		}
		log.Info("update complete", "task", task.Name)
	}
}
