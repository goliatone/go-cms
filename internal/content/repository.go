package content

type Repository interface {
	Save(Content) error
}

type NoOpRepository struct{}

func NewNoOpRepository() Repository {
	return NewNoOpRepository()
}

func (NoOpRepository) Save(Content) error {
	return nil
}
