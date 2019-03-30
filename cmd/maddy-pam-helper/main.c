#define _POSIX_C_SOURCE 200809L
#include <stdio.h>
#include <stdlib.h>
#include <security/pam_appl.h>

/*
I really doubt it is a good idea to bring Go to the binary whose primary task
is to call libpam using CGo anyway.
*/

static struct pam_response *reply;

static int conv_func(int num_msg, const struct pam_message **msg, struct pam_response **resp, void *appdata_ptr) {
    *resp = reply;
    return PAM_SUCCESS;
}

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
        username[username_len-1] = 0;
    }
    if (password_len > 0) {
        password[password_len-1] = 0;
    }

    const struct pam_conv local_conv = { conv_func, NULL };
    pam_handle_t *local_auth = NULL;
    int status = pam_start("maddy", username, &local_conv, &local_auth);
    if (status != PAM_SUCCESS) {
        fprintf(stderr, "pam_start: %s\n", pam_strerror(local_auth, status));
        return 2;
    }

    reply = malloc(sizeof(struct pam_response));
    if (reply == NULL) {
        fprintf(stderr, "malloc returned NULL\n");
        return 2;
    }
    reply->resp = password;
    reply->resp_retcode = 0;
    status = pam_authenticate(local_auth, PAM_SILENT|PAM_DISALLOW_NULL_AUTHTOK);
    if (status != PAM_SUCCESS) {
        if (status == PAM_AUTH_ERR || status == PAM_USER_UNKNOWN) {
            return 1;
        } else {
            fprintf(stderr, "pam_authenticate: %s\n", pam_strerror(local_auth, status));
            return 2;
        }
    }

    status = pam_end(local_auth, status);
    if (status != PAM_SUCCESS) {
            fprintf(stderr, "pam_end: %s\n", pam_strerror(local_auth, status));
            return 2;
    }

    return 0;
}

#ifndef CGO
int main() {
    return run();
}
#endif
