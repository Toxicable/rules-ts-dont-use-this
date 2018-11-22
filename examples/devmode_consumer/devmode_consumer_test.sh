#!/bin/bash
set -e

# should produce named UMD modules
readonly LIBRARY_JS=$(cat $TEST_SRCDIR/build_bazel_rules_typescript/examples/some_library/library.js)
if [[ "$LIBRARY_JS" != *"define(\"some-lib\""* ]]; then
  echo "Expected library.js to declare module named some-lib, but was"
  echo "$LIBRARY_JS"
  exit 1
fi

# should give a name to required modules
readonly BAR_JS=$(cat $TEST_SRCDIR/build_bazel_rules_typescript/examples/bar.js)
if [[ "$BAR_JS" != *"require(\"build_bazel_rules_typescript/examples/foo\")"* ]]; then
  echo "Expected bar.js to require named module foo, but was"
  echo "$BAR_JS"
  exit 1
fi

# should give a name to required modules from other compilation unit
if [[ "$BAR_JS" != *"require(\"some-lib\")"* ]]; then
  echo "Expected bar.js to require named module library, but was"
  echo "$BAR_JS"
  exit 1
fi

# should give a name to required generated modules without bazel-bin
if [[ "$BAR_JS" != *"require(\"build_bazel_rules_typescript/examples/generated_ts/foo\")"* ]]; then
  echo "Expected foo.js to require generated named module foo, but was"
  echo "$BAR_JS"
  exit 1
fi

# should not give a module name to external modules
if [[ "$BAR_JS" != *"require(\"typescript\")"* ]]; then
  echo "Expected foo.js to require typescript by its original name, but was"
  echo "$BAR_JS"
  exit 1
fi

readonly FOO_JS=$(cat $TEST_SRCDIR/build_bazel_rules_typescript/examples/foo.js)
# Should NOT produce es5 code. This does not check if it's es2015/16/16... just only > es5
if [[ "$FOO_JS" != *"class Greeter"* ]]; then
  echo "Expected dev_output.js to contain 'class Greeter', but was"
  echo "$FOO_JS"
  exit 1
fi
