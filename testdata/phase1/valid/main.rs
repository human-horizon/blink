fn main() {
    let x: i32 = 1 + 2;
    let y = x * 3;
    let b = true && false;
    if b {
        let z = 42;
    }
    while x < 10 {
        let z = x + 1;
    }
}

fn add(a: i32, b: i32) -> i32 {
    a + b
}

struct Point { x: i32, y: i32 }

fn origin() -> Point {
    Point { x: 0, y: 0 }
}
