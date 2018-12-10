# Copyright 2018 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Package file which defines build_bazel_rules_typescript version in skylark

check_rules_typescript_version can be used in downstream WORKSPACES to check
against a minimum dependent build_bazel_rules_typescript version.
"""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# This version is synced with the version in package.json.
# It will be automatically synced via the npm "version" script
# that is run when running `npm version` during the release
# process. See `Releasing` section in README.md.
VERSION = "0.22.0"

def rules_typescript_dependencies():
    """
    Fetch our transitive dependencies.

    If the user wants to get a different version of these, they can just fetch it
    from their WORKSPACE before calling this function, or not call this function at all.
    """

    # TypeScript compiler runs on node.js runtime
    _maybe(
        http_archive,
        name = "build_bazel_rules_nodejs",
        urls = ["https://github.com/bazelbuild/rules_nodejs/archive/0.16.1.zip"],
        strip_prefix = "rules_nodejs-0.16.1",
    )

    # ts_web_test depends on the web testing rules to provision browsers.
    _maybe(
        http_archive,
        name = "io_bazel_rules_webtesting",
        urls = ["https://github.com/bazelbuild/rules_webtesting/archive/111d792b9a5b17f87b6e177e274dbbee46094791.zip"],
        strip_prefix = "rules_webtesting-111d792b9a5b17f87b6e177e274dbbee46094791",
        sha256 = "a13af63e928c34eff428d47d31bafeec4e38ee9b6940e70bf2c9cd47184c5c16",
    )

    # ts_devserver depends on the Go rules.
    # See https://github.com/bazelbuild/rules_go#setup for the latest version.
    _maybe(
        http_archive,
        name = "io_bazel_rules_go",
        url = "https://github.com/bazelbuild/rules_go/releases/download/0.16.0/rules_go-0.16.0.tar.gz",
        sha256 = "ee5fe78fe417c685ecb77a0a725dc9f6040ae5beb44a0ba4ddb55453aad23a8a",
    )

    # go_repository is defined in bazel_gazelle
    _maybe(
        http_archive,
        name = "bazel_gazelle",
        urls = ["https://github.com/bazelbuild/bazel-gazelle/archive/109bcfd6880aac2517a1a2d48987226da6337e11.zip"],
        strip_prefix = "bazel-gazelle-109bcfd6880aac2517a1a2d48987226da6337e11",
        sha256 = "8f80ce0f7a6f8a3fee1fb863c9a23e1de99d678c1cf3c6f0a128f3b883168208",
    )

    # ts_auto_deps depends on com_github_bazelbuild_buildtools
    _maybe(
        http_archive,
        name = "com_github_bazelbuild_buildtools",
        url = "https://github.com/bazelbuild/buildtools/archive/0.19.2.1.zip",
        strip_prefix = "buildtools-0.19.2.1",
        sha256 = "9176a7df34dbed2cf5171eb56271868824560364e60644348219f852f593ae79",
    )

def rules_typescript_dev_dependencies():
    """
    Fetch dependencies needed for local development, but not needed by users.

    These are in this file to keep version information in one place, and make the WORKSPACE
    shorter.
    """

    # For running skylint
    _maybe(
        http_archive,
        name = "io_bazel",
        urls = ["https://github.com/bazelbuild/bazel/releases/download/0.17.1/bazel-0.17.1-dist.zip"],
    )

    #############################################
    # Dependencies for generating documentation #
    #############################################

    http_archive(
        name = "io_bazel_rules_sass",
        urls = ["https://github.com/bazelbuild/rules_sass/archive/1.13.4.zip"],
        strip_prefix = "rules_sass-1.13.4",
        sha256 = "5ddde0d3df96978fa537f76e766538c031dee4d29f91a895f4b1345b5e3f9b16",
    )

    http_archive(
        name = "io_bazel_skydoc",
        url = "https://github.com/bazelbuild/skydoc/archive/77e5399258f6d91417d23634fce97d73b40cf337.zip",
        strip_prefix = "skydoc-77e5399258f6d91417d23634fce97d73b40cf337",
        sha256 = "4e9bd9ef65af54dedd997b408fa26c2e70c30ee8e078bcc1b51a33cf7d7f9d7e",
    )

def _maybe(repo_rule, name, **kwargs):
    if name not in native.existing_rules():
        repo_rule(name = name, **kwargs)
