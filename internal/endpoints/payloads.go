package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redhatinsights/payload-tracker-go/internal/config"
	l "github.com/redhatinsights/payload-tracker-go/internal/logging"
	"github.com/redhatinsights/payload-tracker-go/internal/queries"
	"github.com/redhatinsights/payload-tracker-go/internal/structs"
)

var (
	RetrievePayloads          = queries.RetrievePayloads
	RetrieveRequestIdPayloads = queries.RetrieveRequestIdPayloads
	RequestArchiveLink        = requestArchiveLink
	Db                        = getDb
)

var (
	verbosity string = "0"
)

// Payloads returns responses for the /payloads endpoint
func Payloads(w http.ResponseWriter, r *http.Request) {

	// init query with defaults and passed params
	start := time.Now()

	sortBy := r.URL.Query().Get("sort_by")
	incRequests()

	q, err := initQuery(r)

	if err != nil {
		writeResponse(w, http.StatusBadRequest, getErrorBody(fmt.Sprintf("%v", err), http.StatusBadRequest))
		return
	}

	// there is a different default for sortby when searching for payloads
	if sortBy == "" {
		q.SortBy = "created_at"
	}

	if !stringInSlice(q.SortBy, validAllSortBy) {
		message := "sort_by must be one of " + strings.Join(validAllSortBy, ", ")
		writeResponse(w, http.StatusBadRequest, getErrorBody(message, http.StatusBadRequest))
		return
	}
	if !stringInSlice(q.SortDir, validSortDir) {
		message := "sort_dir must be one of " + strings.Join(validSortDir, ", ")
		writeResponse(w, http.StatusBadRequest, getErrorBody(message, http.StatusBadRequest))
		return
	}

	if !validTimestamps(q, false) {
		message := "invalid timestamp format provided"
		writeResponse(w, http.StatusBadRequest, getErrorBody(message, http.StatusBadRequest))
		return
	}

	count, payloads := RetrievePayloads(Db(), q.Page, q.PageSize, q)
	duration := time.Since(start).Seconds()
	observeDBTime(time.Since(start))

	payloadsData := structs.PayloadsData{count, duration, payloads}

	dataJson, err := json.Marshal(payloadsData)
	if err != nil {
		l.Log.Error(err)
		writeResponse(w, http.StatusInternalServerError, getErrorBody("Internal Server Issue", http.StatusInternalServerError))
		return
	}

	writeResponse(w, http.StatusOK, string(dataJson))
}

// RequestIdPayloads returns a response for /payloads/{request_id}
func RequestIdPayloads(w http.ResponseWriter, r *http.Request) {

	reqID := chi.URLParam(r, "request_id")
	verbosity = r.URL.Query().Get("verbosity")

	q, err := initQuery(r)

	if err != nil {
		writeResponse(w, http.StatusBadRequest, getErrorBody(fmt.Sprintf("%v", err), http.StatusBadRequest))
		return
	}

	if !stringInSlice(q.SortBy, validIDSortBy) {
		message := "sort_by must be one of " + strings.Join(validIDSortBy, ", ")
		writeResponse(w, http.StatusBadRequest, getErrorBody(message, http.StatusBadRequest))
		return
	}
	if !stringInSlice(q.SortDir, validSortDir) {
		message := "sort_dir must be one of " + strings.Join(validSortDir, ", ")
		writeResponse(w, http.StatusBadRequest, getErrorBody(message, http.StatusBadRequest))
		return
	}

	payloads := RetrieveRequestIdPayloads(Db(), reqID, q.SortBy, q.SortDir, verbosity)

	if payloads == nil || len(payloads) == 0 {
		writeResponse(w, http.StatusNotFound, getErrorBody("payload with id: "+reqID+" not found", http.StatusNotFound))
		return
	}

	durations := queries.CalculateDurations(payloads)

	payloadsData := structs.PayloadRetrievebyID{Data: payloads, Durations: durations}

	dataJson, err := json.Marshal(payloadsData)
	if err != nil {
		l.Log.Error(err)
		writeResponse(w, http.StatusInternalServerError, getErrorBody("Internal Server Issue", http.StatusInternalServerError))
		return
	}

	writeResponse(w, http.StatusOK, string(dataJson))
}

// PayloadArchiveLink returns a response for /payloads/{request_id}/archiveLink
func PayloadArchiveLink(w http.ResponseWriter, r *http.Request) {

	reqID := chi.URLParam(r, "request_id")

	statusCode, err := checkForRole(r, config.Get().StorageBrokerURLRole)
	if err != nil {
		writeResponse(w, statusCode, getErrorBody(fmt.Sprintf("%v", err), statusCode))
		return
	}

	if !isValidUUID(reqID) {
		IncInvalidAPIRequestIDs()
		writeResponse(w, http.StatusBadRequest, getErrorBody(fmt.Sprintf("%s is not a valid UUID", reqID), http.StatusBadRequest))
		return
	}

	payloadArchiveLink, err := RequestArchiveLink(r, reqID)
	if err != nil {
		l.Log.Errorf("Error getting archive link from storage-broker for request id: %s, error: %v", reqID, err)
		writeResponse(w, http.StatusInternalServerError, getErrorBody(fmt.Sprintf("%v", err), http.StatusInternalServerError))
		return
	}

	if payloadArchiveLink.Url == "" {
		writeResponse(w, http.StatusNotFound, getErrorBody("Payload not found", http.StatusNotFound))
		return
	}

	dataJson, err := json.Marshal(payloadArchiveLink)
	if err != nil {
		l.Log.Error(err)
		writeResponse(w, http.StatusInternalServerError, getErrorBody("Error converting parsed response to json", http.StatusInternalServerError))
		return
	}

	l.Log.Infof("Link generated for payload %s from identity %s: %s", reqID, r.Header.Get("x-rh-identity"), string(dataJson))
	writeResponse(w, http.StatusOK, string(dataJson))
}
