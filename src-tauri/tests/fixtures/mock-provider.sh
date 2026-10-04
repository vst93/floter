#!/bin/sh
# The provider body is data, not an inode. A test writes its snippet to the
# path in $FLOTER_MOCK_PROVIDER_BODY and this committed script sources it.
# Writing a script and exec'ing it is the ETXTBSY class `3ace35b` fixed for the
# probe fixtures (a child forked by another test inherits the write fd and
# holds it until its own exec), so the exec'd file is never written at run time.
. "$FLOTER_MOCK_PROVIDER_BODY"
