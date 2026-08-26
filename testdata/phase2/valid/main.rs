struct Pair<T, U> {
    first: T,
    second: U,
}

fn identity<T>(x: T) -> T {
    x
}

fn make_pair<T, U>(a: T, b: U) -> Pair<T, U> {
    Pair { first: a, second: b }
}

fn main() -> i32 {
    let p: Pair<i32, bool> = Pair { first: 1, second: true };
    let q = make_pair(2, false);
    let r = identity(q.first);
    r
}
