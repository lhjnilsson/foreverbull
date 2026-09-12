def test_fb_database(fb_database):
    engine, verify_or_populate = fb_database
    assert engine is not None
    assert callable(verify_or_populate)
