struct Point {
    x: i32,
    y: i32,
}

impl Point {
    fn new(x: i32, y: i32) -> Point {
        Point { x: x, y: y }
    }

    fn distance(&self) -> i32 {
        self.x + self.y
    }
}

trait Printable {
    fn print(&self) -> i32;
}

impl Printable for Point {
    fn print(&self) -> i32 {
        self.distance()
    }
}

fn main() {
    let p: Point = Point::new(1, 2);
    p.print();
    p.distance();
}
