#pragma once

struct error_obj {
    int status;
    const char* func_name;
    const char* error_msg;
};

struct error_obj run_pam_auth(const char *username, char *password);
