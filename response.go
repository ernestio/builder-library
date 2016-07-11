/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package scheduler

import (
	"bytes"
	"encoding/json"
	"log"
)

func (s *Scheduler) updateFromResponse(body []byte) request {
	raw := json.RawMessage(body)
	res := s.mapJson(&raw)

	json.Unmarshal(body, &res)
	req := request{}
	batchID := res["_batch_id"].(string)

	stored, _ := s.R.Get(batchID).Result()
	json.Unmarshal([]byte(stored), &req)
	for i, c := range req.Components {
		sc := s.mapJson(c)
		if sc["_uuid"] == res["_uuid"] {
			b := json.RawMessage(body)
			req.Components[i] = &b
		}
	}
	body, _ = json.Marshal(req)
	s.R.Set(batchID, body, 0)

	if s.isAllDone(req) {
		req.Status = "completed"
		body, _ := json.Marshal(req)
		s.R.Set(batchID, body, 0)
	}

	return req
}

func (s *Scheduler) mapJson(c *json.RawMessage) (sc map[string]interface{}) {
	dec := json.NewDecoder(bytes.NewReader(*c))
	dec.Decode(&sc)

	return sc
}

func (s *Scheduler) isAllDone(req request) bool {
	for _, c := range req.Components {
		m := s.mapJson(c)
		if m["status"] != "completed" {
			return false
		}
	}
	return true
}

func (s *Scheduler) processNext(req request) *json.RawMessage {
	for _, c := range req.Components {
		sc := s.mapJson(c)
		if sc["status"] == nil || sc["status"] == "" {
			return c
		}
	}

	return nil
}

func (s *Scheduler) manageSuccessResponse(body []byte, nex string, done string) {
	req := s.updateFromResponse(body)

	if req.Status == "completed" {
		body, _ = json.Marshal(req)
		s.N.Publish(done, body)
		return
	}

	if next := s.processNext(req); next != nil {
		s.publishNext(nex, next)
	}
}

func (s *Scheduler) areAllProcessed() {
}

func (s *Scheduler) manageFailedResponse(body []byte, to string) {
	req := s.updateFromResponse(body)
	if body, err := json.Marshal(req); err != nil {
		log.Println(err.Error())
	} else {
		s.N.Publish(to, body)
	}
}
