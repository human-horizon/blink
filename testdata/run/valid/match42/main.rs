enum Status {
    Active,
    Blocked(i32),
}

fn code(s: Status) -> i32 {
    match s {
        Status::Active => 7,
        Status::Blocked(c) => c,
    }
}

fn classify(n: i32) -> i32 {
    match n {
        1 | 2 => 10,
        10..=20 => 20,
        _ => 30,
    }
}

fn loops() -> i32 {
    let mut t: i32 = 0;
    let pairs = [(2, 2), (1, 2)];
    for (p, q) in pairs {
        t = t + p + q;
    }
    let mut s: i32 = 0;
    for i in 0..3 {
        s = s + 1;
    }
    t + s
}

fn main() -> i32 {
    let a = code(Status::Blocked(10));
    let b = classify(2);
    let t = (3, 4);
    let c = match t {
        (3, y) => y,
        _ => 0,
    };
    let d = match Status::Active {
        Status::Active => 1,
        Status::Blocked(_) => 2,
    };
    let e = code(Status::Active);
    let f = loops();
    a + b + c + d + e + f
}
