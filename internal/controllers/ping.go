package controllers

import (
	"net/http"
	"time"

	"github.com/NodeFactoryIo/vedran/internal/auth"
	"github.com/NodeFactoryIo/vedran/internal/models"
	log "github.com/sirupsen/logrus"
)

const (
	pingIntervalSeconds = 10
)

func (c ApiController) PingHandler(w http.ResponseWriter, r *http.Request) {
	request := r.Context().Value(auth.RequestContextKey).(*auth.RequestContext)

	lastPingTime, downtimeDuration, err := c.pingRepo.CalculateDowntime(request.NodeId)
	if err != nil {
		log.Errorf("Unable to calculate node downtime, error: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if downtimeDuration.Seconds() > pingIntervalSeconds {
		downtime := models.Downtime{
			Start:  lastPingTime,
			End:    time.Now(),
			NodeId: request.NodeId,
		}
		err = c.downtimeRepo.Save(&downtime)
		if err != nil {
			log.Errorf("Unable to save node downtime, error: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// save ping to database
	ping := models.Ping{
		NodeId:    request.NodeId,
		Timestamp: request.Timestamp,
	}
	err = c.pingRepo.Save(&ping)
	if err != nil {
		log.Errorf("Unable to save ping %v to database, error: %v", ping, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Debugf("Ping from node %s", ping.NodeId)
}
