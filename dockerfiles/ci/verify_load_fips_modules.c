#include <openssl/provider.h>

// Taken from https://www.openssl.org/docs/man3.0/man7/fips_module.html
int main(void)
{
    OSSL_PROVIDER *fips;
    OSSL_PROVIDER *base;

    fips = OSSL_PROVIDER_load(NULL, "fips");
    if (fips == NULL) {
        printf("Failed to load FIPS provider\n");
        exit(1);
    }
    base = OSSL_PROVIDER_load(NULL, "base");
    if (base == NULL) {
        OSSL_PROVIDER_unload(fips);
        printf("Failed to load base provider\n");
        exit(1);
    }

    /* Rest of application */

    OSSL_PROVIDER_unload(base);
    OSSL_PROVIDER_unload(fips);
    exit(0);
}
