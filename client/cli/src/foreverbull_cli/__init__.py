from multiprocessing import set_start_method


try:
    set_start_method("spawn")
except RuntimeError:
    pass
