struct Point {
    x: i32,
    y: i32,
}

trait Printable {
    fn print(&self) -> i32;
    fn extra(&self) -> i32;
}

impl Printable for Point {
    fn print(&self) -> i32 {
        0
    }
}

fn main() {
    let p: Point = Point { x: 0, y: 0 };
    p.print();
}
