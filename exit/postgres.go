package exit

func (cmd *Command) decAccBalance(uid string, amt int) (rem int, err error) {
	// grab a TX at the database
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	// get the current value
	rw := tx.QueryRow("SELECT Mbs FROM AccBalances WHERE Uid = $1", uid)
	err = rw.Scan(&rem)
	if err != nil {
		return
	}
	// we don't really care about whether the remaining fails or succeeds
	// we deduct amt from rem
	rem -= amt
	if rem < 0 {
		rem = 0
	}
	// set the thing in the database to rem
	tx.Exec("UPDATE AccBalances SET Mbs = $1 WHERE Uid = $2", rem, uid)
	tx.Commit()
	return
}
