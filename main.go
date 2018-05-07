package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	jsonhandler "github.com/apex/log/handlers/json"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/tj/go/http/response"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

// TODO: Maybe put env variables in this this config struct too
type handler struct{ db *sql.DB }

// {{DOMAIN}}/api/pending-invitations?accessToken={{API_ACCESS_TOKEN}}
type invite struct {
	ID         string `json:"_id"`
	InvitedBy  int    `json:"invitedBy"`
	Invitee    int    `json:"invitee"`
	Role       string `json:"role"`
	IsOccupant bool   `json:"isOccupant"`
	CaseID     int    `json:"caseId"`
	UnitID     int    `json:"unitId"`
	Type       string `json:"type"`
}

func init() {
	if os.Getenv("UP_STAGE") == "" {
		log.SetHandler(text.Default)
	} else {
		log.SetHandler(jsonhandler.Default)
	}
}

func main() {

	db, err := sql.Open("mysql", os.Getenv("DSN"))
	if err != nil {
		log.WithError(err).Fatal("error opening database")
	}

	defer db.Close()

	h := handler{db: db}
	addr := ":" + os.Getenv("PORT")
	http.Handle("/run", http.HandlerFunc(h.runProc))
	http.Handle("/", http.HandlerFunc(h.handleInvite))
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.WithError(err).Fatal("error listening")
	}

}

func (h handler) lookupRoleID(roleName string) (id_role_type int, err error) {
	err = h.db.QueryRow("SELECT id_role_type FROM ut_role_types WHERE role_type=?", roleName).Scan(&id_role_type)
	return id_role_type, err
}

func (h handler) processInvite(invites []invite) (result error) {

	for _, invite := range invites {
		log.Infof("Processing invite: %+v", invite)

		roleID, err := h.lookupRoleID(invite.Role)
		if err != nil {
			result = multierror.Append(result, multierror.Prefix(err, invite.ID))
			continue
		}
		log.Infof("%s role converted to id: %d", invite.Role, roleID)

		_, err = h.db.Exec(
			`INSERT INTO ut_invitation_api_data (mefe_invitation_id,
			bzfe_invitor_user_id,
			bz_user_id,
			user_role_type_id,
			is_occupant,
			bz_case_id,
			bz_unit_id,
			invitation_type,
			is_mefe_only_user,
			user_more
		) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			invite.ID,
			invite.InvitedBy,
			invite.Invitee,
			roleID,
			invite.IsOccupant,
			invite.CaseID,
			invite.UnitID,
			invite.Type,
			1,
			"Use Unee-T for a faster reply",
		)
		if err != nil {
			result = multierror.Append(result, multierror.Prefix(err, invite.ID))
			continue
		}

	}

	return result

}

func getInvites() (lr []invite, err error) {
	resp, err := http.Get(os.Getenv("DOMAIN") + "/api/pending-invitations?accessToken=" + os.Getenv("API_ACCESS_TOKEN"))
	if err != nil {
		return lr, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&lr)
	return lr, err
}

func markInvitesProcessed(ids []string) (err error) {

	jids, err := json.Marshal(ids)
	if err != nil {
		log.WithError(err).Error("marshalling")
		return err
	}

	// log.Infof("IDs: %s", jids)

	payload := strings.NewReader(string(jids))

	url := os.Getenv("DOMAIN") + "/api/pending-invitations/done?accessToken=" + os.Getenv("API_ACCESS_TOKEN")
	req, err := http.NewRequest("PUT", url, payload)
	if err != nil {
		log.WithError(err).Error("making PUT")
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Cache-Control", "no-cache")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.WithError(err).Error("PUT request")
		return err
	}

	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.WithError(err).Error("reading body")
		return err
	}

	i, err := strconv.Atoi(string(body))
	if err != nil {
		log.WithError(err).Error("reading body")
		return err
	}

	//log.Infof("Response: %v", res)
	//log.Infof("Num: %d", i)
	//log.Infof("Body: %s", string(body))
	if i != len(ids) {
		return fmt.Errorf("Acted on %d invitations, but %d were submitted", i, len(ids))
	}

	return

}

func (h handler) handleInvite(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("X-Robots-Tag", "none") // We don't want Google to index us

	invites, err := getInvites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("Input %+v", invites)

	err = h.processInvite(invites)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response.OK(w, "Worked")

}

func (h handler) runProc(w http.ResponseWriter, r *http.Request) {

	var outArg string
	_, err := h.db.Exec("CALL ProcName")
	if err != nil {
		log.WithError(err).Error("running proc")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response.OK(w, outArg)

}
