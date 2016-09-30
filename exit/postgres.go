package exit

/* SCHEMA

CREATE TABLE RemBw (
	Uid TEXT PRIMARY KEY,
	Mbs INTEGER NOT NULL CHECK (Mbs > 0)
)

*/

func (cmd *Command) decAccBalance(uid string, amt int) (rem int, err error) {
	// grab a TX at the database
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	// get the current value
	rw := tx.QueryRow("SELECT Mbs FROM RemBw WHERE Uid = $1", uid)
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
	tx.Exec("UPDATE RemBw SET Mbs = $1 WHERE Uid = $2", rem, uid)
	tx.Commit()
	return
}
