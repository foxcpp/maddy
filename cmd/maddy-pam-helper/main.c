#define _POSIX_C_SOURCE 200809L
#include <stdio.h>
#include <stdlib.h>
#include <security/pam_appl.h>
#include "pam.h"

/*
I really doubt it is a good idea to bring Go to the binary whose primary task
is to call libpam using CGo anyway.
*/

int run(void) {
    char *username = NULL, *password = NULL;
    size_t username_buf_len = 0, password_buf_len = 0;

    ssize_t username_len = getline(&username, &username_buf_len, stdin);
    if (username_len < 0) {
        perror("getline username");
        return 2;
    }

    ssize_t password_len = getline(&password, &password_buf_len, stdin);
    if (password_len < 0) {
        perror("getline password");
        return 2;
    }

    // Cut trailing \n.
    if (username_len > 0) {
        username[username_len - 1] = 0;
    }
    if (password_len > 0) {
        password[password_len - 1] = 0;
    }

    struct error_obj err = run_pam_auth(username, password);
    if (err.status != 0) {
        if (err.status == 2) {
            fprintf(stderr, "%s: %s\n", err.func_name, err.error_msg);
        }
        return err.status;
    }

    return 0;
}

#ifndef CGO
int main() {
    return run();
}
#endif
