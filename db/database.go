package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

const (
	PRESIGN_STATUS_NOT_USED = "not_used"
	PRESIGN_STATUS_USED     = "used"
)

type Database interface {
	Init() error

	SavePreparams(chain string, preparams *keygen.LocalPreParams) error
	LoadPreparams(chain string) (*keygen.LocalPreParams, error)

	SaveKeygenData(chain string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error
	LoadKeygenData(chain, workId string) (*keygen.LocalPartySaveData, error)

	SavePresignData(chain string, workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error
	GetAvailablePresignShortForm() ([]string, []string, []int, error)

	LoadPresign(workId string, batchIndexes []int) ([]*presign.LocalPresignData, error)
	UpdatePresignStatus(workId string, batchIndexes []int) error
}

type dbLogger struct {
}

func (loggger *dbLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func (loggger *dbLogger) Verbose() bool {
	return true
}

// --- /

// SqlDatabase implements Database interface.
type SqlDatabase struct {
	db     *sql.DB
	config *config.DbConfig
}

func NewDatabase(config *config.DbConfig) Database {
	return &SqlDatabase{
		config: config,
	}
}

func (d *SqlDatabase) Connect() error {
	host := d.config.Host
	if host == "" {
		return fmt.Errorf("DB host cannot be empty")
	}

	username := d.config.Username
	password := d.config.Password
	schema := d.config.Schema

	// Connect to the db
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/", username, password, host, d.config.Port))
	if err != nil {
		return err
	}
	_, err = database.Exec("CREATE DATABASE IF NOT EXISTS " + schema)
	if err != nil {
		return err
	}
	database.Close()

	database, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, d.config.Port, schema))
	if err != nil {
		return err
	}

	d.db = database
	utils.LogInfo("Db is connected successfully")
	return nil
}

func (d *SqlDatabase) DoMigration() error {
	driver, err := mysql.WithInstance(d.db, &mysql.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		d.config.MigrationPath,
		"mysql",
		driver,
	)

	if err != nil {
		return err
	}

	m.Log = &dbLogger{}
	m.Up()

	return nil
}

func (d *SqlDatabase) Init() error {
	err := d.Connect()
	if err != nil {
		utils.LogError("Failed to connect to DB. Err =", err)
		return err
	}

	err = d.DoMigration()
	if err != nil {
		utils.LogError("Cannot do migration. Err =", err)
		return err
	}

	return nil
}

func (d *SqlDatabase) SavePreparams(chain string, preparams *keygen.LocalPreParams) error {
	bz, err := json.Marshal(preparams)
	if err != nil {
		return err
	}

	query := "INSERT preparams(chain, preparams) VALUES (?, ?)"
	_, err = d.db.Exec(query, chain, bz)

	return err
}

func (d *SqlDatabase) LoadPreparams(chain string) (*keygen.LocalPreParams, error) {
	query := "SELECT preparams FROM preparams where chain=?"

	rows, err := d.db.Query(query, chain)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, fmt.Errorf("There is no preparams.")
	}

	var bz []byte
	err = rows.Scan(&bz)
	if err != nil {
		return nil, err
	}

	preparams := &keygen.LocalPreParams{}
	err = json.Unmarshal(bz, preparams)
	if err != nil {
		return nil, err
	}

	return preparams, nil
}

func (d *SqlDatabase) SaveKeygenData(chain string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error {
	if len(keygenOutput) == 0 {
		return nil
	}

	pidString := getPidString(pids)
	// Constructs multi-insert query to do all insertion in 1 query.
	query := "INSERT keygen (chain, work_id, pids_string, batch_index, keygen_output) VALUES "
	query = query + getQueryQuestionMark(len(keygenOutput), 5)

	params := make([]interface{}, 0)
	for i, output := range keygenOutput {
		bz, err := json.Marshal(output)
		if err != nil {
			return err
		}

		params = append(params, chain)
		params = append(params, workId)
		params = append(params, pidString)
		params = append(params, i) // batch index
		params = append(params, bz)
	}

	_, err := d.db.Exec(query, params...)

	return err
}

func (d *SqlDatabase) LoadKeygenData(chain, workId string) (*keygen.LocalPartySaveData, error) {
	query := "SELECT keygen_output FROM keygen WHERE chain=? AND work_id=? AND batch_index=0"

	params := []interface{}{
		chain,
		workId,
	}

	rows, err := d.db.Query(query, params...)
	if err != nil {
		return nil, err
	}

	result := &keygen.LocalPartySaveData{}

	if rows.Next() {
		var bz []byte

		rows.Scan(&bz)
		err := json.Unmarshal(bz, result)
		if err != nil {
			return nil, err
		}
	} else {
		utils.LogVerbose("There is no such keygen output for ", chain, workId)
	}

	return result, nil
}

func (d *SqlDatabase) SavePresignData(chain string, workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error {
	if len(presignOutputs) == 0 {
		return nil
	}

	pidString := getPidString(pids)

	// Constructs multi-insert query to do all insertion in 1 query.
	query := "INSERT INTO presign (chain, work_id, pids_string, batch_index, status, presign_output) VALUES "
	query = query + getQueryQuestionMark(len(presignOutputs), 6)

	params := make([]interface{}, 0)
	for i, output := range presignOutputs {
		bz, err := json.Marshal(output)
		if err != nil {
			return err
		}

		params = append(params, chain)
		params = append(params, workId)
		params = append(params, pidString)

		params = append(params, i) // batch_index
		params = append(params, PRESIGN_STATUS_NOT_USED)

		params = append(params, bz)
	}

	_, err := d.db.Exec(query, params...)

	return err
}

// GetAllPresignIndexes returns all available presign data sets in short form (pids, workId, index)
// We don't want to load full data of presign sets since it might take too much memmory.
func (d *SqlDatabase) GetAvailablePresignShortForm() ([]string, []string, []int, error) {
	query := "SELECT work_id, batch_index, pids_string FROM presign WHERE status='not_used'"

	pids := make([]string, 0)
	workIds := make([]string, 0)
	indexes := make([]int, 0)

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, nil, nil, err
	}

	for {
		if rows.Next() {
			var id, workId string
			var index int

			rows.Scan(&id, &workId, &index)
			pids = append(pids, id)
			workIds = append(workIds, workId)
			indexes = append(indexes, index)
		} else {
			break
		}
	}

	return pids, workIds, indexes, nil
}

// This is not part of Database interface. Should ony be used in testing since we don't want to delete
// presign rows.
func (d *SqlDatabase) DeletePresignWork(workId string) error {
	_, err := d.db.Exec("DELETE FROM presign WHERE work_id = ?", workId)
	return err
}

func (d *SqlDatabase) DeleteKeygenWork(workId string) error {
	_, err := d.db.Exec("DELETE FROM keygen WHERE work_id = ?", workId)
	return err
}

func (d *SqlDatabase) LoadPresign(workId string, batchIndexes []int) ([]*presign.LocalPresignData, error) {
	// 1. Constract the query
	questions := ""
	for i := range batchIndexes {
		questions = questions + "?"
		if i < len(batchIndexes)-1 {
			questions = questions + ","
		}
	}

	query := "SELECT work_id, batch_index, pids_string, presign_output FROM presign WHERE status='not_used' AND work_id IN = ? AND batch_index IN (" + questions + ") ORDER BY created_time DESC"

	// Execute the query
	interfaceArr := make([]interface{}, len(batchIndexes))
	for i, s := range batchIndexes {
		interfaceArr[i] = s
	}

	rows, err := d.db.Query(query, interfaceArr...)

	if err != nil {
		return nil, err
	}

	// 2. Scan every rows and save it to loaded array
	results := make([]*presign.LocalPresignData, 0)
	for {
		if rows.Next() {
			var pid, workId string
			var batchIndex int
			var bz []byte
			rows.Scan(&workId, &batchIndex, &pid, &bz)

			var data *presign.LocalPresignData
			err := json.Unmarshal(bz, data)
			if err != nil {
				utils.LogError("Cannot unmarshall data")
			} else {
				results = append(results, data)
			}
		} else {
			break
		}
	}

	return results, nil
}

func (d *SqlDatabase) UpdatePresignStatus(workId string, batchIndexes []int) error {
	batchIndexString := ""

	for i := range batchIndexes {
		batchIndexString = batchIndexString + "?"
		if i < len(batchIndexes)-1 {
			batchIndexString = batchIndexString + ","
		}
	}

	query := "UPDATE status FROM presign WHERE work_id = ? AND batch_index IN (" + batchIndexString + ")"

	interfaceArr := make([]interface{}, 0)
	interfaceArr = append(interfaceArr, workId)
	for _, index := range batchIndexes {
		interfaceArr = append(interfaceArr, index)
	}

	_, err := d.db.Exec(query, interfaceArr...)
	return err
}
