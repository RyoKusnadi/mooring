# noisy prints what a verbose test run of a big repository would: about 12000
# lines, of which one reports a real failure.
.PHONY: noisy
noisy:
	@awk 'BEGIN { \
		for (p = 1; p <= 60; p++) { \
			pkg = sprintf("fixture/internal/pkg%02d", p); \
			if (p == 41) pkg = "fixture/internal/ledger"; \
			printf "=== RUN   TestSuite\n"; \
			for (i = 1; i <= 66; i++) { \
				printf "=== RUN   TestSuite/case%03d\n", i; \
				printf "    suite_test.go:%d: checking %s case %d, input ok\n", 10 + i, pkg, i; \
				printf "--- PASS: TestSuite/case%03d (0.00s)\n", i; \
			} \
			if (pkg == "fixture/internal/ledger") { \
				printf "=== RUN   TestRoundTrip\n"; \
				printf "    ledger_test.go:88: total after reload = 41, want 42\n"; \
				printf "--- FAIL: TestRoundTrip (0.00s)\n"; \
				printf "FAIL\nFAIL\t%s\t0.014s\n", pkg; \
			} else { \
				printf "PASS\nok  \t%s\t0.0%02ds\n", pkg, p % 90; \
			} \
		} \
	}'
