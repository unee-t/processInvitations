package main

import (
	"encoding/json"
	"net/http"
	"os"

	jsonhandler "github.com/apex/log/handlers/json"
	"github.com/tj/go/http/response"

	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/gorilla/pat"

	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type listInvitesResponse []struct {
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
	addr := ":" + os.Getenv("PORT")
	app := pat.New()
	app.Get("/", processPendingInvites)
	if err := http.ListenAndServe(addr, app); err != nil {
		log.WithError(err).Fatal("error listening")
	}

}

func set(lr listInvitesResponse) error {

	db, err := sql.Open("mysql", os.Getenv("DSN"))
	if err != nil {
		return err
	}

	defer db.Close()

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		log.WithError(err).Error("failed to open database")
		return err
	}

	for _, invite := range lr {
		log.Infof("Processing invite: %+v", invite)

		result, err := db.Exec(
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
			invite.Role,
			invite.IsOccupant,
			invite.CaseID,
			invite.UnitID,
			invite.Type,
			1,
			"Use Unee-T for a faster reply",
		)
		if err != nil {
			return err
		}

		log.Infof("Exec result %v", result)

	}

	return err

}

func getInvites() (lr listInvitesResponse, err error) {
	resp, err := http.Get(os.Getenv("DOMAIN") + "/api/pending-invitations?accessToken=" + os.Getenv("API_ACCESS_TOKEN"))
	if err != nil {
		return lr, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&lr)
	return lr, err
}

func processPendingInvites(w http.ResponseWriter, r *http.Request) {

	if os.Getenv("UP_STAGE") != "production" {
		w.Header().Set("X-Robots-Tag", "none")
	}

	// Input
	lr, err := getInvites()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("Input %+v", lr)

	err = set(lr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response.OK(w, "Worked")

}
