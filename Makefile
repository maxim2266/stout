FILES := stout.go
TEST_FILES := stout_test.go

TEST := test-result

test: $(TEST)

$(TEST): $(FILES) $(TEST_FILES)
	goimports -w $?
	go test | tee $@

clean:
	rm -f $(TEST)
