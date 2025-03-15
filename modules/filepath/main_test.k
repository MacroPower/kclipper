test_filepath_base = lambda {
    path = base("/a/b/c")
    assert path == "c"
}

test_filepath_clean = lambda {
    path = clean("/a/b/c/../d")
    assert path == "/a/b/d"
}

test_filepath_dir = lambda {
    path = dir("/a/b/c")
    assert path == "/a/b"
}

test_filepath_ext = lambda {
    path = ext("/a/b/c.txt")
    assert path == ".txt"
}

test_filepath_join = lambda {
    path = join(["/a", "b", "c"])
    assert path == "/a/b/c"
}

test_filepath_split = lambda {
    path = split("/a/b/c")
    assert path == ["/a/b/", "c"]
}
