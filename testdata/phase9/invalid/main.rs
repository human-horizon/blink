trait Speak {
    fn speak(&self);
}

struct Cat {}

fn call<T: Speak>(x: T) {}

fn main() {
    let c = Cat;
    call(c);
}
