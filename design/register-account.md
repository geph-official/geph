### Account registration workflow

The workflow for registering an account is a bit strange (and hacky...).

Essentially, the UI would call `geph proxbinder`. `geph proxbinder` will output one line to stdout:

````
127.0.0.1:some-port
````

This is where a mirror of the binder will live. For example `http://127.0.0.1:12345/exit-info` will act as a reverse proxy for `https://binder.geph.io/exit-info` (through the CDN, of course).

`proxbinder` is useful mostly for accessing the binder in a censorship-resistant way without valid Geph credentials, for example, when registering accounts!

In addition, `proxbinder` provides an additional method called `/derive-keys`, which is not present at the binder, that is crucial to the registering accounts.

The exact process of registering a new account is as follows:

 - Spawn `geph proxbinder` and wait until it prints the address it's listening on.
 - Obtain a captcha through `/fresh-captcha` (see binder docs) and display it, together with the username/password form in the UI.
 - After the user fills in the form and clicks "register", do `GET /derive-keys?uname=...&pwd=...`. The response will be a JSON object; get the value of the `PubKey` field.
 - Register the account through `/register-account`, using the `PubKey` derived in the previous step.
 - Kill the subprocess

 Everything described in this document is already implemented.
