# SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

FROM eu.gcr.io/gardener-project/3rd/alpine:3.12.1

COPY bin/rel/docforge-linux-amd64 /usr/local/bin/docforge

WORKDIR /
