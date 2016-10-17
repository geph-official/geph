package client

import "log"

// smVerifyAccount is the VerifyAccount state where the account info is verified.
// => SteadyState if the account info is okay
// => BadAuth if the account info is not okay
func (cmd *Command) smVerifyAccount() {
	log.Println("** => VerifyAccount **")
	defer log.Println("** <= VerifyAccount **")
	// TODO this is currently a no-op that goes straight to SteadyState
	cmd.smState = cmd.smSteadyState
}
