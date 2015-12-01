package main

import (
	"github.com/opinionated/scraper-core/scraper"
	"github.com/opinionated/utils/log"
	"os"
	"time"
)

type rssMonitor struct {
	oldArticles map[string]bool
	rss         scraper.RSS
	tracker     *rateTracker
}

func (task rssMonitor) update() {
	if changed, _ := task.didChange(); changed {
		task.tracker.update()
	}
}

func (task rssMonitor) didChange() (bool, error) {
	err := scraper.UpdateRSS(task.rss)
	if err != nil {
		log.Error("error reading rss:", err)
		return false, err
	}

	// mark all articles as not in list
	for key := range task.oldArticles {
		task.oldArticles[key] = false
	}

	// an article is new if it wasn't in the last RSS ping
	found := false
	for i := 0; i < task.rss.GetChannel().GetNumArticles(); i++ {
		article := task.rss.GetChannel().GetArticle(i)

		if _, inOld := task.oldArticles[article.GetLink()]; !inOld {
			found = true
		}

		// add or update what we found
		task.oldArticles[article.GetLink()] = true
	}

	// remove any articles not in the set
	for key, inList := range task.oldArticles {
		if !inList {
			delete(task.oldArticles, key)
		}
	}

	if found {
		log.Info("found new article")
	}
	return found, nil
}

func newMonitor(rss scraper.RSS) rssMonitor {
	return rssMonitor{make(map[string]bool), rss, newRateTracker()}
}

// seperate class so testing is easy
type rateTracker struct {
	last        time.Time
	averageTime float64
	updateCount float64
}

func newRateTracker() *rateTracker {
	return &rateTracker{time.Now(), 0.0, 1}
}

func (t *rateTracker) update() {
	since := time.Since(t.last)

	t.averageTime = (t.averageTime * (t.updateCount / (t.updateCount + 1))) + (since.Seconds() / (t.updateCount + 1.0))
	t.updateCount++
	t.last = time.Now()
}

func (t *rateTracker) getAverage() float64 {
	return t.averageTime
}

func updateFeeds(feeds []rssMonitor) {
	for _, monitor := range feeds {
		monitor.update()
	}
}

func main() {
	infoFile, err := os.OpenFile("rateInfoLog.txt", os.O_RDWR|os.O_CREATE, 0666)
	errFile, err := os.OpenFile("rateErrorLog.txt", os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}

	defer infoFile.Close()
	defer errFile.Close()

	log.Init(infoFile, nil, errFile)

	feeds := []rssMonitor{newMonitor(&scraper.WSJRSS{}),
		newMonitor(&scraper.NYTRSS{})}

	// get whats currently in the feeds without sending an update signal
	for _, monitor := range feeds {
		monitor.didChange()
	}

	ticker := time.NewTicker(time.Duration(5) * time.Minute)

	// flip only prints the articles once every 5 minutes
	flip := false

	for {
		// wait for new ticker value
		<-ticker.C

		updateFeeds(feeds)

		if flip {
			for _, monitor := range feeds {
				log.Info("for feed:", monitor.rss,
					"average time is: ", monitor.tracker.getAverage())
			}
		}
		flip = !flip
	}

}
