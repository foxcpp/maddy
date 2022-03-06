//+build libpam

/*
Maddy Mail Server - Composable all-in-one email server.
Copyright © 2019-2022 Max Mazurov <fox.cpp@disroot.org>, Maddy Mail Server contributors

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

#define _POSIX_C_SOURCE 200809L
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <security/pam_appl.h>
#include "pam.h"

static int conv_func(int num_msg, const struct pam_message **msg, struct pam_response **resp, void *appdata_ptr) {
    struct pam_response *reply = malloc(sizeof(struct pam_response));
    if (reply == NULL) {
        return PAM_CONV_ERR;
    }

    char* password_cpy = malloc(strlen((char*)appdata_ptr)+1);
    if (password_cpy == NULL) {
        return PAM_CONV_ERR;
    }
    memcpy(password_cpy, (char*)appdata_ptr, strlen((char*)appdata_ptr)+1);

    reply->resp = password_cpy;
    reply->resp_retcode = 0;

    // PAM frees pam_response for us.
    *resp = reply;

    return PAM_SUCCESS;
}

struct error_obj run_pam_auth(const char *username, char *password) {
    const struct pam_conv local_conv = { conv_func, password };
    pam_handle_t *local_auth = NULL;
    int status = pam_start("maddy", username, &local_conv, &local_auth);
    if (status != PAM_SUCCESS) {
        struct error_obj ret_val;
        ret_val.status = 2;
        ret_val.func_name = "pam_start";
        ret_val.error_msg = pam_strerror(local_auth, status);
        return ret_val;
    }

    status = pam_authenticate(local_auth, PAM_SILENT|PAM_DISALLOW_NULL_AUTHTOK);
    if (status != PAM_SUCCESS) {
        struct error_obj ret_val;
        if (status == PAM_AUTH_ERR || status == PAM_USER_UNKNOWN) {
            ret_val.status = 1;
        } else {
            ret_val.status = 2;
        }
        ret_val.func_name = "pam_authenticate";
        ret_val.error_msg = pam_strerror(local_auth, status);
        return ret_val;
    }

    status = pam_acct_mgmt(local_auth, PAM_SILENT|PAM_DISALLOW_NULL_AUTHTOK);
    if (status != PAM_SUCCESS) {
        struct error_obj ret_val;
        if (status == PAM_AUTH_ERR || status == PAM_USER_UNKNOWN || status == PAM_NEW_AUTHTOK_REQD) {
            ret_val.status = 1;
        } else {
            ret_val.status = 2;
        }
        ret_val.func_name = "pam_acct_mgmt";
        ret_val.error_msg = pam_strerror(local_auth, status);
        return ret_val;
    }

    status = pam_end(local_auth, status);
    if (status != PAM_SUCCESS) {
        struct error_obj ret_val;
        ret_val.status = 2;
        ret_val.func_name = "pam_end";
        ret_val.error_msg = pam_strerror(local_auth, status);
        return ret_val;
    }

    struct error_obj ret_val;
    ret_val.status = 0;
    ret_val.func_name = NULL;
    ret_val.error_msg = NULL;
    return ret_val;
}

